package main

import (
	"os"

	"github.com/veandco/go-sdl2/sdl"
)

const winHeight = 600

func main() {
	s := "PulseAudio JACK Sink"
	if len(os.Args) >= 2 {
		// system:capture_1
		// XMMS2
		s = os.Args[1]
	}
	j, err := NewJack(s)
	if err != nil {
		panic(err)
	}
	defer j.Close()

	if err := sdl.Init(sdl.INIT_TIMER | sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		panic(err)
	}
	defer sdl.Quit()

	fw, err := NewFFTWindow(j.SourceName(), j.Samples(), winHeight)
	if err != nil {
		panic(err)
	}
	defer fw.Close()
	fw.Run()
}
