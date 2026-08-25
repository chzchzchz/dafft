package main

import (
	"image/color"
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func sinRow(hz, fs, amp, phase float64, n int) []float32 {
	row := make([]float32, n)
	for i := range row {
		row[i] = float32(amp * math.Sin(2*math.Pi*hz*float64(i)/fs+phase))
	}
	return row
}

func argmax(v []float32) int {
	m, mi := v[0], 0
	for i, x := range v[1:] {
		if x > m {
			m, mi = x, i+1
		}
	}
	return mi
}

// specRow drives the production SpectrogramChan pipeline over one
// full-length chunk and returns the single dB row.
func specRow(t *testing.T, row []float32) []float32 {
	t.Helper()
	inc := make(chan []float32, 1)
	outc := SpectrogramChan(inc, len(row), 1)
	inc <- row
	close(inc)
	got, ok := <-outc
	if !ok {
		t.Fatal("no spectrogram row produced")
	}
	if _, ok := <-outc; ok {
		t.Fatal("expected output channel to close after input close")
	}
	return got
}

func TestHann(t *testing.T) {
	n := 1024
	w := hann(n)
	if len(w) != n {
		t.Fatalf("hann length = %d, want %d", len(w), n)
	}
	if w[0] != 0 {
		t.Errorf("hann start = %v, want 0", w[0])
	}
	if float64(w[n-1]) > 1e-4 {
		t.Errorf("periodic wrap endpoint %v too large", w[n-1])
	}
	max := float32(0)
	for i := range w {
		if !near(float64(w[i]), float64(w[(n-i)%n]), 1e-6) {
			t.Fatalf("hann violates periodic symmetry at %d", i)
		}
		if w[i] > max {
			max = w[i]
		}
	}
	if !near(float64(max), 1.0, 1e-6) {
		t.Errorf("hann peak = %v, want ~1", max)
	}
}

// dftMagDB is a naive reference DFT with the same normalization as
// magnitudeDB; comparing the two validates the R2HC halfcomplex unpacking,
// including DC and Nyquist slots.
func dftMagDB(x []float32) []float32 {
	n := len(x)
	out := make([]float32, n/2+1)
	for k := range out {
		var re, im float64
		for i := 0; i < n; i++ {
			angle := -2 * math.Pi * float64(k) * float64(i) / float64(n)
			re += float64(x[i]) * math.Cos(angle)
			im += float64(x[i]) * math.Sin(angle)
		}
		scale := 2.0 / float64(n)
		if k == 0 || 2*k == n {
			scale = 1.0 / float64(n)
		}
		out[k] = float32(20 * math.Log10(math.Hypot(re, im)*scale+dbEpsilon))
	}
	return out
}

func TestMagnitudeMatchesDFT(t *testing.T) {
	const n = 16
	x := sinRow(3.0*44100.0/n, 44100.0, 0.75, 0.3, n)
	for i := 0; i < n; i++ {
		x[i] += float32(0.25 * math.Cos(2*math.Pi*float64(5*i)/float64(n)))
	}
	plan := NewPlan(n)
	defer plan.Destroy()
	got := magnitudeDB(plan.Execute(x))
	want := dftMagDB(x)
	for k := range want {
		if want[k] < -140 {
			continue // dbEpsilon floor: dominated by float32 vs float64 rounding
		}
		if !near(float64(got[k]), float64(want[k]), 0.01) {
			t.Errorf("bin %d: got %.4fdB, want %.4fdB", k, got[k], want[k])
		}
	}
}

func TestDCDominance(t *testing.T) {
	row := make([]float32, 2048)
	for i := range row {
		row[i] = 1
	}
	spec := specRow(t, row)
	// Hann-windowed DC lands exactly at -6.02dB; its own spectrum puts equal
	// normalized energy in bins 0 and +/-1, so only bound leakage further out.
	if !near(float64(spec[0]), -6.02, 0.5) {
		t.Errorf("DC bin = %.2fdB, want ~-6dB", spec[0])
	}
	if spec[1] > spec[0]+0.1 {
		t.Errorf("bin 1 = %.2fdB exceeds DC %.2fdB", spec[1], spec[0])
	}
	for k := 8; k < len(spec); k++ {
		if spec[k] > -30 {
			t.Errorf("bin %d = %.2fdB, want < -30dB", k, spec[k])
		}
	}
}

func TestSinePeakLocation(t *testing.T) {
	spec := specRow(t, sinRow(440, float64(sampHz), 1, 0, fftSize))
	wantBin := int(math.Round(440.0 * fftSize / float64(sampHz)))
	peak := argmax(spec)
	if math.Abs(float64(peak-wantBin)) > 1 {
		t.Errorf("peak bin = %d, want %d +/- 1", peak, wantBin)
	}
	if !near(float64(spec[peak]), -6, 2) {
		t.Errorf("peak = %.2fdB, want ~-6dB for unit amplitude", spec[peak])
	}
	for d := 4; d <= 16; d++ {
		for _, k := range []int{peak - d, peak + d} {
			if k < 0 || k >= len(spec) {
				continue
			}
			if spec[k] > spec[peak]-25 {
				t.Errorf("bin %d only %.1fdB below peak, want >= 25dB", k, spec[peak]-spec[k])
			}
		}
	}
}

func TestPhaseInvariance(t *testing.T) {
	a := specRow(t, sinRow(440, float64(sampHz), 1, 0, fftSize))
	b := specRow(t, sinRow(440, float64(sampHz), 1, math.Pi/2, fftSize))
	for k := range a {
		if a[k] < -100 {
			continue // skirt/noise-floor bins dominated by float32 rounding
		}
		if !near(float64(a[k]), float64(b[k]), 0.1) {
			t.Errorf("bin %d phase dependent: %.3f vs %.3f dB", k, a[k], b[k])
		}
	}
}

func TestDbLinearity(t *testing.T) {
	loud := specRow(t, sinRow(440, float64(sampHz), 1, 0, fftSize))
	quiet := specRow(t, sinRow(440, float64(sampHz), 0.25, 0, fftSize))
	const wantDelta = 12.04 // 20*log10(4)
	peak := argmax(loud)
	for k := peak - 2; k <= peak+2; k++ {
		delta := loud[k] - quiet[k]
		if !near(float64(delta), wantDelta, 0.5) {
			t.Errorf("bin %d: delta = %.2fdB, want %.2f", k, delta, wantDelta)
		}
	}
}

func TestHopAccounting(t *testing.T) {
	const chunks = 10
	const chunkLen = 1024
	inc := make(chan []float32, 1)
	outc := SpectrogramChan(inc, fftSize, fftSplit)
	go func() {
		defer close(inc)
		for i := 0; i < chunks; i++ {
			inc <- make([]float32, chunkLen)
		}
	}()
	rows := 0
	for r := range outc {
		if len(r) != fftSize/2+1 {
			t.Fatalf("row length = %d, want %d", len(r), fftSize/2+1)
		}
		rows++
	}
	if rows != fftSplit*chunks {
		t.Errorf("rows = %d, want %d", rows, fftSplit*chunks)
	}
}

func TestImpulseSpansWindow(t *testing.T) {
	const (
		n     = 256
		split = 2
		chunk = 128
		zeros = 8
	)
	hop := chunk / split
	spans := n / hop
	inc := make(chan []float32, 1)
	outc := SpectrogramChan(inc, n, split)
	impulse := make([]float32, chunk)
	impulse[32] = 1000
	go func() {
		defer close(inc)
		inc <- impulse
		for i := 0; i < zeros; i++ {
			inc <- make([]float32, chunk)
		}
	}()
	total, hot := 0, 0
	for r := range outc {
		total++
		if r[argmax(r)] > -60 {
			hot++
		}
	}
	if total != split*(zeros+1) {
		t.Errorf("total rows = %d, want %d", total, split*(zeros+1))
	}
	if hot != spans {
		t.Errorf("impulse appeared in %d rows, want exactly %d (one slide per hop)", hot, spans)
	}
}

func TestRemainderNotDropped(t *testing.T) {
	const (
		n         = 256
		split     = 2
		chunk     = 127 // indivisible: the final sample used to be discarded
		impulses  = 5
		gapChunks = 3
	)
	inc := make(chan []float32, 1)
	outc := SpectrogramChan(inc, n, split)
	go func() {
		defer close(inc)
		for c := 0; c < impulses; c++ {
			row := make([]float32, chunk)
			row[chunk-1] = 1000
			inc <- row
			for g := 0; g < gapChunks; g++ {
				inc <- make([]float32, chunk)
			}
		}
	}()
	clusters, prevHot := 0, false
	for r := range outc {
		isHot := r[argmax(r)] > -60
		if isHot && !prevHot {
			clusters++
		}
		prevHot = isHot
	}
	if clusters != impulses {
		t.Errorf("saw %d impulse clusters, want %d (final-sample remainder dropped)", clusters, impulses)
	}
}

func TestFFTBin2Color(t *testing.T) {
	cases := []struct {
		v    float32
		want color.NRGBA
	}{
		{0, color.NRGBA{0, 0, 0, 255}},
		{float32(math.NaN()), color.NRGBA{0, 0, 0, 255}},
		{-1, color.NRGBA{0, 0, 0, 255}},
		{1, color.NRGBA{255, 255, 255, 255}},
		{4, color.NRGBA{255, 255, 255, 255}},
	}
	for _, c := range cases {
		if got := FFTBin2Color(c.v); got != c.want {
			t.Errorf("FFTBin2Color(%v) = %v, want %v", c.v, got, c.want)
		}
	}
	mid := FFTBin2Color(0.5)
	if mid.G != 255 || mid.B != 0 || mid.R < 100 || mid.R > 155 {
		t.Errorf("mid ramp color = %v, want green-yellow blend", mid)
	}
}
