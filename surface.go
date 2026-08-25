package main

import (
	"image/color"
	//"math"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

type fftTexture struct {
	r       *sdl.Renderer
	rows    []*sdl.Texture
	rowIdx  int // wraps around
	w       int
	row8888 []byte
	rowRect *sdl.Rect
}

func newFFTTexture(r *sdl.Renderer, w, h int) *fftTexture {
	ft := &fftTexture{
		r:       r,
		rows:    make([]*sdl.Texture, h),
		w:       w,
		row8888: make([]byte, w*4),
		rowRect: &sdl.Rect{X: 0, Y: 0, W: int32(w), H: 1},
	}
	for i := 0; i < w; i++ {
		ft.row8888[4*i+3] = 0xff
	}
	// Create textures for each row.
	for i := range ft.rows {
		var err error
		ft.rows[i], err = r.CreateTexture(
			sdl.PIXELFORMAT_RGB888, sdl.TEXTUREACCESS_STREAMING, int32(w), 1)
		if err != nil {
			panic(err)
		}
		if err = ft.rows[i].Update(ft.rowRect, unsafe.Pointer(&ft.row8888[0]), 4); err != nil {
			panic(err)
		}
	}
	return ft
}

func (ft *fftTexture) Destroy() {
	for _, r := range ft.rows {
		r.Destroy()
	}
	ft.rows = nil
}

func (ft *fftTexture) blit() {
	dstRect := &sdl.Rect{X: 0, Y: 0, W: int32(ft.w), H: 1}
	for i := ft.rowIdx; i < len(ft.rows); i++ {
		if err := ft.r.Copy(ft.rows[i], ft.rowRect, dstRect); err != nil {
			panic(err)
		}
		dstRect.Y++
	}
	for i := 0; i < ft.rowIdx; i++ {
		if err := ft.r.Copy(ft.rows[i], ft.rowRect, dstRect); err != nil {
			panic(err)
		}
		dstRect.Y++
	}
}

func (ft *fftTexture) add(row []float32) {
	span := dbMax - dbMin
	for i, v := range row {
		norm := (v - dbMin) / span
		if norm < 0 {
			norm = 0
		} else if norm > 1 {
			norm = 1
		}
		c := FFTBin2Color(norm)
		ft.row8888[4*i] = byte(c.R)
		ft.row8888[4*i+1] = byte(c.G)
		ft.row8888[4*i+2] = byte(c.B)
	}
	ft.rows[ft.rowIdx].Update(ft.rowRect, unsafe.Pointer(&ft.row8888[0]), 4)

	ft.rowIdx++
	if ft.rowIdx >= len(ft.rows) {
		ft.rowIdx = 0
	}
}

// black, green, yellow, white
var colorScale = []color.NRGBA{
	{0, 0, 0, 255},
	{0, 255, 0, 255},
	{255, 255, 0, 255},
	{255, 255, 255, 255},
}

// Display range for dB-magnitude rows.
const (
	dbMin = float32(-100)
	dbMax = float32(0)
)

func interpolate(t float32, a, b uint8) uint8 { return uint8(float32(a)*(1-t) + float32(b)*t) }

func FFTBin2Color(v float32) color.NRGBA {
	idx := float32(len(colorScale)-1) * v
	if !(v >= 0) {
		idx = 0
	} else if v >= 1 {
		idx = float32(len(colorScale) - 1)
	}
	ii := int(idx)
	if ii >= len(colorScale)-1 {
		return colorScale[len(colorScale)-1]
	}
	t := idx - float32(ii)
	prev, next := colorScale[ii], colorScale[ii+1]
	return color.NRGBA{
		interpolate(t, prev.R, next.R),
		interpolate(t, prev.G, next.G),
		interpolate(t, prev.B, next.B),
		255,
	}
}
