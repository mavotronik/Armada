package gpio

import (
	"context"
	"sync"
	"time"
)

type swPWM struct {
	cancel context.CancelFunc
}

type softwarePWM struct {
	mu   sync.Mutex
	pins map[string]*swPWM
	write func(linuxGPIO int, high bool) error
}

func newSoftwarePWM(write func(linuxGPIO int, high bool) error) *softwarePWM {
	return &softwarePWM{
		pins:  map[string]*swPWM{},
		write: write,
	}
}

func (s *softwarePWM) stop(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.pins[id]; ok {
		p.cancel()
		delete(s.pins, id)
	}
}

func (s *softwarePWM) stopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, p := range s.pins {
		p.cancel()
		delete(s.pins, id)
	}
}

func (s *softwarePWM) start(id string, linuxGPIO int, frequencyHz, dutyPercent float64) {
	s.stop(id)

	if dutyPercent <= 0 {
		_ = s.write(linuxGPIO, false)
		return
	}
	if dutyPercent >= 100 {
		_ = s.write(linuxGPIO, true)
		return
	}
	if frequencyHz < 1 {
		frequencyHz = 1
	}
	if frequencyHz > 2000 {
		frequencyHz = 2000
	}

	period := time.Duration(float64(time.Second) / frequencyHz)
	high := time.Duration(float64(period) * dutyPercent / 100)
	low := period - high
	if low < time.Microsecond {
		low = time.Microsecond
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.pins[id] = &swPWM{cancel: cancel}
	s.mu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_ = s.write(linuxGPIO, true)
			if !sleepOrDone(ctx, high) {
				return
			}
			_ = s.write(linuxGPIO, false)
			if !sleepOrDone(ctx, low) {
				return
			}
		}
	}()
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
