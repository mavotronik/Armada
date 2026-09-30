//go:build linux

package gpio

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func sysfsPWMEnabled(pwm *PWMDef) bool {
	if pwm == nil {
		return false
	}
	base := pwmBase(pwm)
	if _, err := os.Stat(base); err != nil {
		return false
	}
	exported := filepath.Join(base, fmt.Sprintf("pwm%d", pwm.Channel))
	if _, err := os.Stat(exported); err == nil {
		return true
	}
	_ = writeSysfs(filepath.Join(base, "export"), strconv.Itoa(pwm.Channel))
	_, err := os.Stat(exported)
	return err == nil
}

func sysfsSetHardwarePWM(pwm *PWMDef, frequencyHz, dutyPercent float64) error {
	if pwm == nil {
		return fmt.Errorf("no hardware pwm")
	}
	if frequencyHz < 1 {
		frequencyHz = 1
	}
	if dutyPercent < 0 {
		dutyPercent = 0
	}
	if dutyPercent > 100 {
		dutyPercent = 100
	}
	base := pwmBase(pwm)
	exported := filepath.Join(base, fmt.Sprintf("pwm%d", pwm.Channel))
	if _, err := os.Stat(exported); err != nil {
		if err := writeSysfs(filepath.Join(base, "export"), strconv.Itoa(pwm.Channel)); err != nil {
			return err
		}
	}
	periodNs := int64(1e9 / frequencyHz)
	dutyNs := int64(float64(periodNs) * dutyPercent / 100)
	if err := writeSysfs(filepath.Join(exported, "enable"), "0"); err != nil {
		return err
	}
	if err := writeSysfs(filepath.Join(exported, "period"), strconv.FormatInt(periodNs, 10)); err != nil {
		return err
	}
	if err := writeSysfs(filepath.Join(exported, "duty_cycle"), strconv.FormatInt(dutyNs, 10)); err != nil {
		return err
	}
	return writeSysfs(filepath.Join(exported, "enable"), "1")
}

func sysfsStopHardwarePWM(pwm *PWMDef) error {
	if pwm == nil {
		return nil
	}
	sysfsReleasePWMChip(pwm.Chip)
	return nil
}

// sysfsReleasePWMChip disables and unexports every PWM channel on a pwmchip (frees GPIO mux).
func sysfsReleasePWMChip(chip int) {
	base := pwmBase(&PWMDef{Chip: chip})
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if !strings.HasPrefix(ent.Name(), "pwm") || ent.Name() == "export" || ent.Name() == "unexport" {
			continue
		}
		chPath := filepath.Join(base, ent.Name())
		_ = writeSysfs(filepath.Join(chPath, "enable"), "0")
		num := strings.TrimPrefix(ent.Name(), "pwm")
		_ = writeSysfs(filepath.Join(base, "unexport"), num)
	}
}
