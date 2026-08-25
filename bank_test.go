package main

import (
	"testing"
)

func TestBankLinearCoversRange(t *testing.T) {
	hzPerBin := sampHz / fftSize
	b := NewBankLinear(hzPerBin, minHz, maxHz, fftWinDiv)
	wantW := ((maxHz / hzPerBin) - (minHz / hzPerBin)) / fftWinDiv
	if b.Width() != wantW {
		t.Errorf("Width = %d, want %d", b.Width(), wantW)
	}
	if len(b.buckets) != 1 || b.buckets[0] <= 0 {
		t.Fatalf("linear buckets = %v, want single positive bucket", b.buckets)
	}
	if b.Width()%b.buckets[0] != 0 {
		t.Errorf("Width %d not divisible by bucket width %d", b.Width(), b.buckets[0])
	}
	row := make([]float32, fftSize/2+1)
	toneBin := 1000 / hzPerBin // 1000 Hz tone
	row[toneBin] = 100
	out := b.apply(row)
	if argmax(out) != toneBin/fftWinDiv {
		t.Errorf("1000Hz tone landed at bucket %d, want %d", argmax(out), toneBin/fftWinDiv)
	}
}

func TestBankEqualTemperment(t *testing.T) {
	hzPerBin := sampHz / fftSize
	b := NewBankEqualTemperment(hzPerBin, 49 /* G1 */, 12*6)
	if b.Width() != 12*6 {
		t.Errorf("Width = %d, want %d", b.Width(), 12*6)
	}
	if b.minBin <= 0 || b.maxBin <= b.minBin || b.maxBin > fftSize/2+1 {
		t.Errorf("bin range [%d,%d) outside spectrum", b.minBin, b.maxBin)
	}
	prev := 0
	for _, n := range b.buckets {
		if n <= 0 {
			t.Fatalf("bucket width %d not positive", n)
		}
		if n < prev {
			t.Errorf("buckets not nondecreasing: %d after %d", n, prev)
		}
		prev = n
	}
	row := make([]float32, fftSize/2+1)
	for i := range row {
		row[i] = float32(i)
	}
	out := b.apply(row)
	if len(out) != b.Width() {
		t.Errorf("apply length = %d, want %d", len(out), b.Width())
	}
}
