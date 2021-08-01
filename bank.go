package main

import (
	"fmt"
	"math"
)

type Bank struct {
	minHz    int
	maxHz    int
	buckets  []int
	outSamps int
}

func (b *Bank) apply(in []float32) []float32 {
	out := make([]float32, b.outSamps)
	i, k := 0, 0
	inSlice := in[b.minHz:b.maxHz]
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

func NewBankLinear(insize, min, max, div int) *Bank {
	b := make([]int, 1)
	b[0] = div
	return &Bank{
		minHz:    min,
		maxHz:    max,
		buckets:  b,
		outSamps: (max - min) / div,
	}
}

func (b *Bank) Width() int { return b.outSamps }

func NewBankEqualTemperment(insize, start, steps int) *Bank {
	b := make([]int, steps)
	midf, max := float64(start), 0.0
	for n := 0; n < steps; n++ {
		lstep := midf * math.Pow(2, float64(n-1)/12.0)
		rstep := midf * math.Pow(2, float64(n+1)/12.0)
		delta := (rstep - lstep) / 2.0
		fmt.Println(delta, math.Ceil(delta))
		b[n] = int(math.Ceil(delta))
		max = midf + delta
	}
	lstep := midf * math.Pow(2, float64(-1)/12.0)
	return &Bank{
		minHz:    int((midf + lstep) / 2.0),
		maxHz:    start + int(max),
		buckets:  b,
		outSamps: steps,
	}
}
