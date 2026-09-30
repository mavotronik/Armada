package gpio

import "sync"

// StubBackend simulates GPIO for dev/Docker without sysfs.
type StubBackend struct {
	mu      sync.Mutex
	levels  map[int]bool
	exports map[int]bool
	dirs    map[int]Mode
}

func NewStubBackend() *StubBackend {
	return &StubBackend{
		levels:  map[int]bool{},
		exports: map[int]bool{},
		dirs:    map[int]Mode{},
	}
}

func (b *StubBackend) Hardware() bool { return false }

func (b *StubBackend) Export(linuxGPIO int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.exports[linuxGPIO] = true
	return nil
}

func (b *StubBackend) Unexport(linuxGPIO int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.exports, linuxGPIO)
	delete(b.dirs, linuxGPIO)
	delete(b.levels, linuxGPIO)
	return nil
}

func (b *StubBackend) SetDirection(linuxGPIO int, mode Mode) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dirs[linuxGPIO] = mode
	return nil
}

func (b *StubBackend) Read(linuxGPIO int) (bool, error) {
	return b.ReadLevel(linuxGPIO)
}

func (b *StubBackend) ReadLevel(linuxGPIO int) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.levels[linuxGPIO], nil
}

func (b *StubBackend) Write(linuxGPIO int, high bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.levels[linuxGPIO] = high
	return nil
}

func (b *StubBackend) HardwarePWMEnabled(_ *PWMDef) bool { return false }

func (b *StubBackend) SetHardwarePWM(_ *PWMDef, _, _ float64) error { return nil }

func (b *StubBackend) StopHardwarePWM(_ *PWMDef) error { return nil }
