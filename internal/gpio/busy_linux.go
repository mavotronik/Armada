//go:build linux

package gpio

import (
	"fmt"
	"strings"

	"github.com/warthog618/go-gpiocdev"
)

func PinByLinuxGPIO(n int) (PinDef, bool) {
	for _, p := range Catalog {
		if p.LinuxGPIO() == n {
			return p, true
		}
	}
	return PinDef{}, false
}

func enrichGPIOBusyError(chip string, offset, linuxGPIO int, err error) error {
	if err == nil {
		return nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "busy") {
		return err
	}
	base := err
	var parts []string

	var consumer, kernelName string
	if info, infoErr := readLineInfo(chip, offset); infoErr == nil {
		consumer = info.Consumer
		kernelName = info.Name
		if consumer != "" {
			parts = append(parts, fmt.Sprintf("consumer=%q", consumer))
		}
		if kernelName != "" {
			parts = append(parts, fmt.Sprintf("kernel name=%q", kernelName))
		}
		if info.Used && consumer == "" {
			parts = append(parts, "линия помечена as used")
		}
	}

	def, hasDef := PinByLinuxGPIO(linuxGPIO)
	if hasDef && def.Header > 0 {
		parts = append(parts, fmt.Sprintf("контакт %d (%s)", def.Header, def.Name))
	}

	cLower := strings.ToLower(consumer)
	switch {
	case strings.Contains(cLower, "spi"):
		parts = append(parts, "SPI держит линию — в device tree: status=\"disabled\" у &spi0 (и дочернего spidev), пересоберите DTB/ядро; пока SPI нужен, используйте другой контакт (11, 17, 29, 34)")
	case strings.Contains(cLower, "pwm"):
		if hasDef && def.HardwarePWM != nil {
			parts = append(parts, fmt.Sprintf("отключите PWM (pwmchip%d) в device tree или /sys/class/pwm/pwmchip%d/*", def.HardwarePWM.Chip, def.HardwarePWM.Chip))
		}
	case strings.Contains(cLower, "uart"):
		if hasDef && def.UART != "" {
			parts = append(parts, fmt.Sprintf("отключите %s в device tree", def.UART))
		}
	default:
		if hasDef && def.MuxNote != "" {
			parts = append(parts, def.MuxNote)
		} else if hasDef && def.HardwarePWM != nil {
			parts = append(parts, fmt.Sprintf("возможен аппаратный PWM (pwmchip%d) — проверьте device tree", def.HardwarePWM.Chip))
		}
	}

	parts = append(parts, "диагностика: apk add libgpiod && gpioinfo "+chip+"  (consumer уже в логе armada)")

	if len(parts) == 0 {
		return base
	}
	return fmt.Errorf("%v (%s)", base, strings.Join(parts, "; "))
}

func readLineInfo(chip string, offset int) (gpiocdev.LineInfo, error) {
	c, err := gpiocdev.NewChip(chip)
	if err != nil {
		return gpiocdev.LineInfo{}, err
	}
	defer c.Close()
	return c.LineInfo(offset)
}
