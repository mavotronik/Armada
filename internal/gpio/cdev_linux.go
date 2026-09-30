//go:build linux

package gpio

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/warthog618/go-gpiocdev"
)

// CdevBackend drives GPIO through the Linux GPIO character device (libgpiod API).
type CdevBackend struct {
	mu    sync.Mutex
	lines map[int]*gpiocdev.Line
	modes map[int]Mode
}

func NewCdevBackend() *CdevBackend {
	return &CdevBackend{
		lines: map[int]*gpiocdev.Line{},
		modes: map[int]Mode{},
	}
}

func (b *CdevBackend) Hardware() bool { return true }

func (b *CdevBackend) Export(linuxGPIO int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.lines[linuxGPIO]; ok {
		return nil
	}
	// Line is opened on SetDirection / Write — avoid briefly claiming as input.
	return nil
}

func (b *CdevBackend) Unexport(linuxGPIO int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	line, ok := b.lines[linuxGPIO]
	if !ok {
		return nil
	}
	err := line.Close()
	delete(b.lines, linuxGPIO)
	delete(b.modes, linuxGPIO)
	return err
}

func (b *CdevBackend) SetDirection(linuxGPIO int, mode Mode) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if cur, ok := b.modes[linuxGPIO]; ok && cur == mode && b.lines[linuxGPIO] != nil {
		return nil
	}
	if line, ok := b.lines[linuxGPIO]; ok {
		_ = line.Close()
		delete(b.lines, linuxGPIO)
	}
	b.modes[linuxGPIO] = mode
	outHigh := false
	if mode == ModeOut {
		outHigh = false
	}
	return b.requestLocked(linuxGPIO, mode, outHigh)
}

func (b *CdevBackend) Read(linuxGPIO int) (bool, error) {
	b.mu.Lock()
	line, ok := b.lines[linuxGPIO]
	mode := b.modes[linuxGPIO]
	b.mu.Unlock()
	if !ok || mode != ModeIn {
		if err := b.SetDirection(linuxGPIO, ModeIn); err != nil {
			return false, err
		}
		b.mu.Lock()
		line = b.lines[linuxGPIO]
		b.mu.Unlock()
	}
	v, err := line.Value()
	return v != 0, err
}

func (b *CdevBackend) ReadLevel(linuxGPIO int) (bool, error) {
	b.mu.Lock()
	line, ok := b.lines[linuxGPIO]
	b.mu.Unlock()
	if !ok {
		return false, fmt.Errorf("line %d not open", linuxGPIO)
	}
	v, err := line.Value()
	return v != 0, err
}

func (b *CdevBackend) Write(linuxGPIO int, high bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	v := 0
	if high {
		v = 1
	}
	if line, ok := b.lines[linuxGPIO]; ok && b.modes[linuxGPIO] == ModeOut {
		return line.SetValue(v)
	}
	if line, ok := b.lines[linuxGPIO]; ok {
		_ = line.Close()
		delete(b.lines, linuxGPIO)
	}
	b.modes[linuxGPIO] = ModeOut
	return b.requestLocked(linuxGPIO, ModeOut, high)
}

func (b *CdevBackend) HardwarePWMEnabled(pwm *PWMDef) bool {
	return sysfsPWMEnabled(pwm)
}

func (b *CdevBackend) SetHardwarePWM(pwm *PWMDef, frequencyHz, dutyPercent float64) error {
	return sysfsSetHardwarePWM(pwm, frequencyHz, dutyPercent)
}

func (b *CdevBackend) StopHardwarePWM(pwm *PWMDef) error {
	return sysfsStopHardwarePWM(pwm)
}

func (b *CdevBackend) requestLocked(linuxGPIO int, mode Mode, outHigh bool) error {
	chip, offset, err := chipForGlobalGPIO(linuxGPIO)
	if err != nil {
		return err
	}
	var line *gpiocdev.Line
	switch mode {
	case ModeOut:
		v := 0
		if outHigh {
			v = 1
		}
		line, err = gpiocdev.RequestLine(chip, offset,
			gpiocdev.AsOutput(v),
			gpiocdev.AsPushPull,
			gpiocdev.AsActiveHigh,
			gpiocdev.WithConsumer("armada"),
		)
	default:
		line, err = gpiocdev.RequestLine(chip, offset,
			gpiocdev.AsInput,
			gpiocdev.WithConsumer("armada"),
		)
	}
	if err != nil {
		wrapped := fmt.Errorf("request %s offset %d (GPIO %d): %w", chip, offset, linuxGPIO, err)
		return enrichGPIOBusyError(chip, offset, linuxGPIO, wrapped)
	}
	if info, infoErr := line.Info(); infoErr == nil {
		log.Printf("gpio: cdev %s+%d (linux %d): kernel name=%q consumer=%q direction=%v",
			chip, offset, linuxGPIO, info.Name, info.Consumer, info.Config.Direction)
	} else {
		log.Printf("gpio: cdev %s+%d (linux %d): configured", chip, offset, linuxGPIO)
	}
	b.lines[linuxGPIO] = line
	b.modes[linuxGPIO] = mode
	return nil
}

func chipForGlobalGPIO(global int) (chip string, offset int, err error) {
	for _, name := range gpiocdev.Chips() {
		base, n, err := chipBaseAndCount(name)
		if err != nil {
			continue
		}
		if global >= base && global < base+n {
			return name, global - base, nil
		}
	}
	bank := global / 32
	off := global % 32
	for _, name := range gpiocdev.Chips() {
		for _, candidate := range []string{
			fmt.Sprintf("gpiochip%d", bank),
			fmt.Sprintf("gpiochip%d", bank*32),
		} {
			if name != candidate {
				continue
			}
			if chipHasLine(name, off) {
				return name, off, nil
			}
		}
	}
	return "", 0, fmt.Errorf("GPIO %d is not available on this board (no matching gpiochip)", global)
}

func chipHasLine(chipName string, offset int) bool {
	if offset < 0 {
		return false
	}
	c, err := gpiocdev.NewChip(chipName)
	if err != nil {
		return false
	}
	defer c.Close()
	return offset < c.Lines()
}

func chipBaseAndCount(chipName string) (base, count int, err error) {
	basePath := filepath.Join("/sys/class/gpio", chipName, "base")
	data, err := os.ReadFile(basePath)
	if err == nil {
		base, err = strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return 0, 0, err
		}
	} else {
		var n int
		if _, scanErr := fmt.Sscanf(chipName, "gpiochip%d", &n); scanErr != nil {
			return 0, 0, scanErr
		}
		base = gpioChipBaseFromName(n)
	}

	c, err := gpiocdev.NewChip(chipName)
	if err != nil {
		return 0, 0, err
	}
	defer c.Close()
	return base, c.Lines(), nil
}
