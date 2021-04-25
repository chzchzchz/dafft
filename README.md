# dafft

A handy FFT streamer for JACK audio.

## Build and Run

Install fftw and sdl development libraries then run:

```sh
go get github.com/chzchzchz/dafft
dafft # defaults to PulseAudio monitor
dafft XMMS2 # XMMS2 jack monitor
dafft XM # substring matching if you want
```
