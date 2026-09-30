package gpio

// Mode is pin direction.
type Mode string

const (
	ModeIn  Mode = "in"
	ModeOut Mode = "out"
)

// OutputKind is how an output pin is driven.
type OutputKind string

const (
	OutputDigital OutputKind = "digital"
	OutputPWM     OutputKind = "pwm"
)

// PWMBackend reports which PWM implementation is active for a pin.
type PWMBackend string

const (
	PWMBackendNone     PWMBackend = ""
	PWMBackendHardware PWMBackend = "hardware"
	PWMBackendSoftware PWMBackend = "software"
	PWMBackendStub     PWMBackend = "stub"
)

// PinConfig is the user-visible configuration for one pin.
type PinConfig struct {
	Mode         Mode       `json:"mode"`
	Output       OutputKind `json:"output,omitempty"`
	Value        bool       `json:"value,omitempty"`
	FrequencyHz  float64    `json:"frequencyHz,omitempty"`
	DutyPercent  float64    `json:"dutyPercent,omitempty"`
}

// PinRuntime is catalog + live state returned by the API.
type PinRuntime struct {
	ID          string     `json:"id"`
	Header      int        `json:"header,omitempty"`
	Label       string     `json:"label"`
	LinuxGPIO   int        `json:"linuxGpio"`
	VoltageNote string     `json:"voltageNote,omitempty"`
	UART        string     `json:"uart,omitempty"`
	UARTLine    string     `json:"uartLine,omitempty"`
	MuxNote     string     `json:"muxNote,omitempty"`
	Configured  bool       `json:"configured"`
	Config      PinConfig  `json:"config"`
	Default     *PinConfig `json:"default,omitempty"`
	Level       *bool      `json:"level,omitempty"`
	PWMBackend  PWMBackend `json:"pwmBackend,omitempty"`
	LastError   string     `json:"lastError,omitempty"`
}

// Snapshot is the full GPIO API response.
type Snapshot struct {
	Hardware bool         `json:"hardware"`
	Pins     []PinRuntime `json:"pins"`
}

// storedPin persists current and default config on disk.
type storedPin struct {
	Current PinConfig  `json:"current"`
	Default *PinConfig `json:"default,omitempty"`
}

type stateFile struct {
	Pins map[string]storedPin `json:"pins"`
}
