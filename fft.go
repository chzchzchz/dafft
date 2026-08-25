package main

import "math"

const dbEpsilon = 1e-9

type spectrogram struct {
	plan      *fftPlan
	window    []float32
	scratch   []float32
	lastSamps []float32
	pending   []float32
	inc       <-chan []float32
	outc      chan<- []float32
	split     int
}

func hann(n int) []float32 {
	w := make([]float32, n)
	for i := range w {
		w[i] = 0.5 * (1 - float32(math.Cos(2*math.Pi*float64(i)/float64(n))))
	}
	return w
}

// magnitudeDB unpacks FFTW R2HC halfcomplex layout:
// out[0]=Re(X0), out[n/2]=Re(X(n/2)), out[k]=Re(Xk), out[n-k]=Im(Xk).
// Returns n/2+1 bins, DC through Nyquist, normalized to dB.
func magnitudeDB(spec []float32) []float32 {
	n := len(spec)
	out := make([]float32, n/2+1)
	for k := range out {
		var mag float64
		switch {
		case k == 0 || 2*k == n:
			mag = math.Abs(float64(spec[k]))
		default:
			mag = math.Hypot(float64(spec[k]), float64(spec[n-k]))
		}
		scale := 2.0 / float64(n)
		if k == 0 || 2*k == n {
			scale = 1.0 / float64(n)
		}
		out[k] = float32(20 * math.Log10(mag*scale+dbEpsilon))
	}
	return out
}

func (sp *spectrogram) run() {
	for sampsFull := range sp.inc {
		sp.pending = append(sp.pending, sampsFull...)
		w := len(sp.pending) / sp.split
		used := w * sp.split
		for i := 0; i < sp.split; i++ {
			samps := sp.pending[w*i : w*(i+1)]
			copy(sp.lastSamps, sp.lastSamps[w:])

			// add new samples
			copy(sp.lastSamps[len(sp.lastSamps)-w:], samps)

			for j, v := range sp.lastSamps {
				sp.scratch[j] = v * sp.window[j]
			}
			sp.outc <- magnitudeDB(sp.plan.Execute(sp.scratch))
		}
		n := copy(sp.pending, sp.pending[used:])
		sp.pending = sp.pending[:n]
	}
}

func SpectrogramChan(inc <-chan []float32, bins int, split int) <-chan []float32 {
	outc := make(chan []float32, split)
	go func() {
		defer close(outc)
		plan := NewPlan(bins)
		defer plan.Destroy()
		sp := spectrogram{
			plan:      plan,
			window:    hann(bins),
			scratch:   make([]float32, bins),
			lastSamps: make([]float32, bins),
			inc:       inc,
			outc:      outc,
			split:     split,
		}
		sp.run()
	}()
	return outc
}
