package main

import (
	"math"
)

type Bank struct {
	minBin   int
	maxBin   int
	buckets  []int
	outSamps int
}

func (b *Bank) apply(in []float32) []float32 {
	out := make([]float32, b.outSamps)
	i, k := 0, 0
	inSlice := in[b.minBin:b.maxBin]
	for i < len(inSlice) && k < len(out) {
		for _, b := range b.buckets {
			v := float32(b)
			for j := 0; j < b && i < len(inSlice) && k < len(out); j++ {
				out[k] += inSlice[i] / v
				i++
			}
			k++
		}
	}
	return out
}

func NewBankLinear(hzPerBin, min, max, div int) *Bank {
	b := make([]int, 1)
	b[0] = div
	minBin, maxBin := min/hzPerBin, max/hzPerBin
	return &Bank{
		minBin:   minBin,
		maxBin:   maxBin,
		buckets:  b,
		outSamps: (maxBin - minBin) / div,
	}
}

func (b *Bank) Width() int { return b.outSamps }

func NewBankEqualTemperment(hzPerBin, start, steps int) *Bank {
	b := make([]int, steps)
	midf, max := float64(start), 0.0
	for n := 0; n < steps; n++ {
		lstep := midf * math.Pow(2, float64(n-1)/12.0)
		rstep := midf * math.Pow(2, float64(n+1)/12.0)
		delta := (rstep - lstep) / 2.0
		b[n] = int(math.Ceil(delta / float64(hzPerBin)))
		max = midf + delta
	}
	lstep := midf * math.Pow(2, float64(-1)/12.0)
	return &Bank{
		minBin:   int((midf + lstep) / 2.0) / hzPerBin,
		maxBin:   start/hzPerBin + int(max)/hzPerBin,
		buckets:  b,
		outSamps: steps,
	}
}
