package gpio

// Backend drives GPIO and optional hardware PWM.
type Backend interface {
	Hardware() bool
	Export(linuxGPIO int) error
	Unexport(linuxGPIO int) error
	SetDirection(linuxGPIO int, mode Mode) error
	Read(linuxGPIO int) (bool, error)
	// ReadLevel returns the line level without changing its direction.
	ReadLevel(linuxGPIO int) (bool, error)
	Write(linuxGPIO int, high bool) error
	HardwarePWMEnabled(pwm *PWMDef) bool
	SetHardwarePWM(pwm *PWMDef, frequencyHz, dutyPercent float64) error
	StopHardwarePWM(pwm *PWMDef) error
}
