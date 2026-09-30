//go:build !linux

package gpio

// OpenBackend always uses the stub on non-Linux platforms.
func OpenBackend(noGPIO bool) Backend {
	_ = noGPIO
	return NewStubBackend()
}
