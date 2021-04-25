package main

// #cgo CFLAGS: -I/usr/local/include
// #cgo LDFLAGS: -L/usr/local/lib -lfftw3f -lm
// #include <fftw3.h>
import "C"

import (
	"sync"
	"unsafe"
)

var planMu sync.Mutex

type fftPlan struct {
	fftw_p C.fftwf_plan
	n      int
	out    []float32
	in     []float32
}

func (p *fftPlan) Execute(data []float32) []float32 {
	C.fftwf_execute_r2r(p.fftw_p,
		(*C.float)(unsafe.Pointer(&data[0])),
		(*C.float)(unsafe.Pointer(&p.out[0])))
	return p.out
}

func (p *fftPlan) Destroy() {
	planMu.Lock()
	C.fftwf_destroy_plan(p.fftw_p)
	planMu.Unlock()
}

func NewPlan(samples int) *fftPlan {
	in, out := make([]float32, samples), make([]float32, samples)
	planMu.Lock()
	p := C.fftwf_plan_r2r_1d(
		C.int(samples),
		(*C.float)(unsafe.Pointer(&in[0])),
		(*C.float)(unsafe.Pointer(&out[0])),
		C.FFTW_R2HC,
		C.uint(C.FFTW_PRESERVE_INPUT|C.FFTW_ESTIMATE))
	planMu.Unlock()
	return &fftPlan{fftw_p: p, n: samples, in: in, out: out}
}
