//go:build linux

package gpio

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SysfsBackend uses /sys/class/gpio and /sys/class/pwm.
type SysfsBackend struct{}

func NewSysfsBackend() *SysfsBackend { return &SysfsBackend{} }

func (b *SysfsBackend) Hardware() bool { return true }

func (b *SysfsBackend) Export(linuxGPIO int) error {
	if _, err := os.Stat("/sys/class/gpio"); err != nil {
		return err
	}
	gpioPath := fmt.Sprintf("/sys/class/gpio/gpio%d", linuxGPIO)
	if _, err := os.Stat(gpioPath); err == nil {
		return nil
	}
	if err := writeSysfs("/sys/class/gpio/export", strconv.Itoa(linuxGPIO)); err != nil {
		if _, statErr := os.Stat(gpioPath); statErr == nil {
			return nil
		}
		return fmt.Errorf("export GPIO %d: %w (pin may be busy in kernel/DT or sysfs export disabled — use a recent armada build with gpio-cdev)", linuxGPIO, err)
	}
	return nil
}

func (b *SysfsBackend) Unexport(linuxGPIO int) error {
	return writeSysfs("/sys/class/gpio/unexport", strconv.Itoa(linuxGPIO))
}

func (b *SysfsBackend) SetDirection(linuxGPIO int, mode Mode) error {
	p := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", linuxGPIO)
	return writeSysfs(p, string(mode))
}

func (b *SysfsBackend) Read(linuxGPIO int) (bool, error) {
	return b.ReadLevel(linuxGPIO)
}

func (b *SysfsBackend) ReadLevel(linuxGPIO int) (bool, error) {
	p := fmt.Sprintf("/sys/class/gpio/gpio%d/value", linuxGPIO)
	data, err := os.ReadFile(p)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(data)) == "1", nil
}

func (b *SysfsBackend) Write(linuxGPIO int, high bool) error {
	p := fmt.Sprintf("/sys/class/gpio/gpio%d/value", linuxGPIO)
	v := "0"
	if high {
		v = "1"
	}
	return writeSysfs(p, v)
}

func (b *SysfsBackend) HardwarePWMEnabled(pwm *PWMDef) bool {
	return sysfsPWMEnabled(pwm)
}

func (b *SysfsBackend) SetHardwarePWM(pwm *PWMDef, frequencyHz, dutyPercent float64) error {
	return sysfsSetHardwarePWM(pwm, frequencyHz, dutyPercent)
}

func (b *SysfsBackend) StopHardwarePWM(pwm *PWMDef) error {
	return sysfsStopHardwarePWM(pwm)
}

func pwmBase(pwm *PWMDef) string {
	return fmt.Sprintf("/sys/class/pwm/pwmchip%d", pwm.Chip)
}

func writeSysfs(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if _, err = f.WriteString(value); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
