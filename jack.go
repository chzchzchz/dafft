package main

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/xthexder/go-jack"
)

var clientName = "dafft"

type Jack struct {
	client  *jack.Client
	portIn  *jack.Port
	portSrc *jack.Port

	sampc      chan []jack.AudioSample
	portc      chan *jack.Port
	connecting bool
	srcPattern string
	wg         sync.WaitGroup
}

func (j *Jack) Samples() <-chan []jack.AudioSample {
	return j.sampc
}

func (j *Jack) process(nFrames uint32) int {
	p := j.portIn
	if j.portSrc == nil {
		return 0
	}
	buf := p.GetBuffer(nFrames)
	out := make([]jack.AudioSample, nFrames)
	copy(out, buf)
	select {
	case j.sampc <- out:
	default:
	}
	return 0
}

func (j *Jack) portRegistration(id jack.PortId, made bool) {
	p := j.client.GetPortById(id)
	name := p.GetName()
	if !made {
		if j.portSrc != nil && name == j.portSrc.GetName() {
			log.Println("unregistered:", name)
			j.portSrc = nil
		}
		return
	}
	if strings.HasPrefix(name, clientName) || !strings.Contains(name, j.srcPattern) {
		log.Println("ignoring non-match:", name)
		return
	}
	if j.portSrc != nil || len(j.portc) > 0 || j.connecting {
		log.Println("ignoring match:", name)
		return
	}
	j.connecting = true
	j.portc <- p
}

func (j *Jack) SourceName() string {
	if j.portIn == nil {
		return j.srcPattern
	}
	return j.portIn.GetName()
}

func (j *Jack) srcPorts(name string) (ret []*jack.Port) {
	for _, port := range j.client.GetPorts(name, "", 0) {
		if !strings.HasPrefix(port, clientName) {
			p := j.client.GetPortByName(port)
			ret = append(ret, p)
		}
	}
	return ret
}

func (j *Jack) connectInput(src *jack.Port) error {
	p := j.portIn
	log.Printf("connecting src=%q(%s) to dst=%q(%s)",
		src.GetName(), src.GetType(), p.GetName(), p.GetType())
	if code := j.client.ConnectPorts(src, p); code != 0 {
		return jack.StrError(code)
	}
	j.connecting, j.portSrc = false, src
	return nil
}

func (j *Jack) logPorts() {
	for _, port := range j.client.GetPorts("", "", 0) {
		p := j.client.GetPortByName(port)
		log.Println("client:", p.GetClientName(), "/", p.GetName())
	}
}

func NewJack(src string) (*Jack, error) {
	client, status := jack.ClientOpen(clientName, jack.NoStartServer)
	if status != 0 {
		return nil, jack.StrError(status)
	}
	j := &Jack{
		client:     client,
		sampc:      make(chan []jack.AudioSample, 2),
		portc:      make(chan *jack.Port, 2),
		srcPattern: src,
	}

	j.logPorts()

	if code := j.client.SetPortRegistrationCallback(j.portRegistration); code != 0 {
		j.client.Close()
		return nil, jack.StrError(code)
	}
	if code := client.SetProcessCallback(j.process); code != 0 {
		j.client.Close()
		return nil, jack.StrError(code)
	}
	if code := client.Activate(); code != 0 {
		j.client.Close()
		return nil, jack.StrError(code)
	}

	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		for p := range j.portc {
			j.connectInput(p)
		}
	}()

	p := j.client.PortRegister(
		fmt.Sprintf("in_%s", src),
		jack.DEFAULT_AUDIO_TYPE,
		jack.PortIsInput|jack.PortIsTerminal,
		8192)
	j.portIn = p

	if srcs := j.srcPorts(src); len(srcs) > 0 {
		log.Println("registering srcs")
		if err := j.connectInput(srcs[0]); err != nil {
			j.Close()
			return nil, err
		}
	} else {
		log.Println("matching port not found; will wait to register")
	}
	return j, nil
}

func (j *Jack) Close() {
	j.client.Deactivate()
	close(j.sampc)
	j.client.Close()
	close(j.portc)
	j.wg.Wait()
}
