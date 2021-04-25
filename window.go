package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/xthexder/go-jack"
)

var sampHz = 44100

type fftWindow struct {
	win   *sdl.Window
	r     *sdl.Renderer
	ft    *fftTexture
	w     int
	h     int
	sampW int

	sampc <-chan []jack.AudioSample
	fftc  <-chan []float32

	pause bool
	lines []int32

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

const resizable = true
const popup = true
const maxHz = 3000
const minHz = 0
const fftWinDiv = 3
const fftTextureWidth = (maxHz - minHz) / fftWinDiv

func NewFFTWindow(name string, sampc <-chan []jack.AudioSample, h int) (fw *fftWindow, err error) {
	if row0 := <-sampc; row0 == nil {
		return nil, fmt.Errorf("failed reading first row of samples")
	} else {
		log.Println("got row samples", len(row0))
	}

	winFlags := uint32(sdl.WINDOW_SHOWN)
	if resizable {
		winFlags |= sdl.WINDOW_RESIZABLE | sdl.WINDOW_OPENGL | sdl.WINDOW_UTILITY
	}
	if popup {
		winFlags |= sdl.WINDOW_UTILITY
	}
	win, e := sdl.CreateWindow(
		name,
		sdl.WINDOWPOS_UNDEFINED,
		sdl.WINDOWPOS_UNDEFINED,
		int32(fftTextureWidth),
		int32(h),
		winFlags)
	if e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			win.Destroy()
		}
	}()

	// Disable letterboxing.
	sdl.SetHint(sdl.HINT_RENDER_LOGICAL_SIZE_MODE, "1")

	r, e := sdl.CreateRenderer(win, -1, sdl.RENDERER_ACCELERATED|sdl.RENDERER_TARGETTEXTURE)
	if e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			r.Destroy()
		}
	}()

	info, err := r.GetInfo()
	if err != nil {
		return nil, err
	}
	if (info.Flags & sdl.RENDERER_ACCELERATED) == 0 {
		log.Println("no hw acceleration")
	}
	if err := r.SetLogicalSize(int32(fftTextureWidth), int32(h)); err != nil {
		return nil, err
	}
	if err := r.SetIntegerScale(false); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &fftWindow{
		win:    win,
		r:      r,
		w:      fftTextureWidth,
		sampW:  sampHz,
		h:      h,
		ft:     newFFTTexture(r, fftTextureWidth, h),
		sampc:  sampc,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (fw *fftWindow) Close() {
	fw.cancel()
	fw.ft.Destroy()
	fw.r.Destroy()
	fw.win.Destroy()
	fw.wg.Wait()
}

func (fw *fftWindow) redraw() {
	// Draw waterfall.
	fw.ft.blit()

	// Draw selection.
	fw.r.SetDrawColor(0xff, 0xd3, 0, 0xff)
	for _, x := range fw.lines {
		fw.r.DrawLine(x, 0, x, int32(fw.h))
	}

	if err := fw.r.Flush(); err != nil {
		panic(err)
	}
	fw.r.Present()
}

func (fw *fftWindow) sample2fft() <-chan []float32 {
	framec := make(chan []float32, 1)
	fw.wg.Add(1)
	go func() {
		defer func() {
			close(framec)
			fw.wg.Done()
		}()
		for {
			select {
			case row, ok := <-fw.sampc:
				if !ok {
					break
				}
				r32 := *(*[]float32)(unsafe.Pointer(&row))
				select {
				case framec <- r32:
				default:
				}
			case <-fw.ctx.Done():
				return
			}
		}
	}()
	return framec
}
func (fw *fftWindow) Run() {
	// FFT uses frame rate limited channel to avoid processing dropped rows.
	fw.fftc = SpectrogramChan(fw.sample2fft(), fw.sampW, 4)
	fps := float64(1 + (44100 / 1024))
	fpsDur := time.Duration(float64(time.Second) / fps)
	ticker := time.NewTicker(fpsDur)
	defer ticker.Stop()
	for fw.processEvents() {
		<-ticker.C
		if fw.pause {
			continue
		}
		fw.updateRows()
		fw.redraw()

		select {
		case <-ticker.C:
		default:
		}
		ticker.Reset(fpsDur)
	}
}

func (fw *fftWindow) updateRows() {
	for len(fw.fftc) > 0 {
		if row := <-fw.fftc; row != nil {
			r := row[minHz:maxHz]
			rr := make([]float32, len(r)/fftWinDiv)
			for i := 0; i < len(r); i++ {
				rr[i/fftWinDiv] += rr[i/fftWinDiv] + row[i]/float32(fftWinDiv)
			}
			fw.ft.add(rr)
		} else {
			log.Println("stream terminated")
			return
		}
	}
}

func (fw *fftWindow) handleEvent(event sdl.Event) bool {
	switch ev := event.(type) {
	case *sdl.QuitEvent:
		return false
	case *sdl.MouseButtonEvent:
		if ev.Type != sdl.MOUSEBUTTONDOWN {
			break
		}
		if ev.Button == sdl.BUTTON_LEFT {
			if len(fw.lines) > 1 {
				fw.lines = nil
			}
			fw.lines = append(fw.lines, ev.X)
		} else if ev.Button == sdl.BUTTON_RIGHT {
			fw.lines = nil
		}
		if fw.pause {
			fw.redraw()
		}
	case *sdl.MouseMotionEvent:
		// cursorMHz := float64(fw.x2hz(ev.X)) / 1.0e6
	case *sdl.WindowEvent:
		if fw.pause {
			fw.redraw()
		}
	case *sdl.KeyboardEvent:
		if ev.Type == sdl.KEYDOWN {
			switch ev.Keysym.Sym {
			case sdl.K_SPACE:
				// TODO: disconnect stream if paused for too long.
				fw.pause = !fw.pause
			case sdl.K_r:
				// Reset window size.
				fw.win.SetSize(int32(fw.w), int32(fw.h))
			}
		} else if ev.Type == sdl.KEYUP {
			switch ev.Keysym.Sym {
			case sdl.K_ESCAPE:
				return false
			}
		}

	}
	return true
}

func (fw *fftWindow) processEvents() bool {
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		if !fw.handleEvent(event) {
			return false
		}
	}
	return true
}
