package main

const filterScale = float32(-4)

type spectrogram struct {
	plan      *fftPlan
	lastSamps []float32
	inc       <-chan []float32
	outc      chan<- []float32
	split     int
}

func (sp *spectrogram) run() {
	for sampsFull := range sp.inc {
		w := len(sampsFull) / sp.split
		// FIR filter, shift down by window.
		for i, v := range sp.lastSamps[len(sampsFull):] {
			sp.lastSamps[i] = v / filterScale
		}
		for i := 0; i < sp.split; i++ {
			samps := sampsFull[w*i : w*(i+1)]
			copy(sp.lastSamps[:len(sp.lastSamps)-w], sp.lastSamps[w:])

			// add new samples
			copy(sp.lastSamps[len(sp.lastSamps)-len(samps):], samps)
			samps = sp.plan.Execute(sp.lastSamps)

			n := make([]float32, len(samps)/2)
			copy(n, samps[:len(samps)/2])
			sp.outc <- n
		}
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
			lastSamps: make([]float32, bins),
			inc:       inc,
			outc:      outc,
			split:     split,
		}
		sp.run()
	}()
	return outc
}
