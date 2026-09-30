//go:build linux

package gpio

import (
	"os"

	"github.com/warthog618/go-gpiocdev"
)

// OpenBackend prefers the GPIO character device; sysfs is a fallback for old kernels.
func OpenBackend(noGPIO bool) Backend {
	if noGPIO {
		return NewStubBackend()
	}
	if len(gpiocdev.Chips()) > 0 {
		return NewCdevBackend()
	}
	if _, err := os.Stat("/sys/class/gpio"); err == nil {
		return NewSysfsBackend()
	}
	return NewStubBackend()
}
