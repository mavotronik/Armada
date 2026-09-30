//go:build !linux

package gpio

// SetIomuxDevice is a no-op on non-Linux builds.
func SetIomuxDevice(path string) {}

func ensurePinMuxGPIO(def PinDef) {}

func gpioBankProbed(bank int) bool { return true }

func forceGPIOLevel(def PinDef, high bool) (string, bool, error) {
	return "", true, nil
}
