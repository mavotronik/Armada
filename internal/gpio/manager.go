package gpio

import (
	"errors"
	"fmt"
	"log"
	"sync"
)

// Manager applies pin configuration and persists state.
type Manager struct {
	mu      sync.Mutex
	store   *Store
	backend Backend
	sw      *softwarePWM
	state   stateFile
	runtime map[string]pinState
}

type pinState struct {
	configured bool
	config     PinConfig
	defaultCfg *PinConfig
	lastError  string
	pwmBackend PWMBackend
}

// NewManager loads state and optionally applies saved pins at startup.
func NewManager(store *Store, noGPIO bool) *Manager {
	backend := OpenBackend(noGPIO)
	st, err := store.Load()
	if err != nil {
		log.Printf("gpio: load state: %v", err)
		st = stateFile{Pins: map[string]storedPin{}}
	}
	m := &Manager{
		store:   store,
		backend: backend,
		sw:      newSoftwarePWM(backend.Write),
		state:   st,
		runtime: map[string]pinState{},
	}
	for id, sp := range st.Pins {
		m.runtime[id] = pinState{
			configured: true,
			config:     sp.Current,
			defaultCfg: cloneConfig(sp.Default),
		}
	}
	for id, ps := range m.runtime {
		if err := m.applyLocked(id, ps.config, "restore"); err != nil {
			ps.lastError = err.Error()
			m.runtime[id] = ps
		}
	}
	return m
}

func (m *Manager) Hardware() bool {
	return m.backend.Hardware()
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := Snapshot{Hardware: m.backend.Hardware()}
	for _, def := range Catalog {
		ps := m.runtime[def.ID]
		cfg := ps.config
		if !ps.configured {
			cfg = PinConfig{Mode: ModeIn}
		}
		pr := PinRuntime{
			ID:          def.ID,
			Header:      def.Header,
			Label:       def.Label(),
			LinuxGPIO:   def.LinuxGPIO(),
			VoltageNote: def.Voltage,
			UART:        def.UART,
			UARTLine:    def.UARTLine,
			MuxNote:     def.MuxNote,
			Configured:  ps.configured,
			Config:      cfg,
			Default:     cloneConfig(ps.defaultCfg),
			PWMBackend:  ps.pwmBackend,
			LastError:   ps.lastError,
		}
		if ps.configured && cfg.Mode == ModeIn && m.backend.Hardware() {
			if v, err := m.readLevel(def); err == nil {
				pr.Level = &v
			}
		} else if ps.configured && cfg.Mode == ModeOut && cfg.Output == OutputDigital {
			v := cfg.Value
			pr.Level = &v
		}
		out.Pins = append(out.Pins, pr)
	}
	return out
}

func (m *Manager) SetConfig(id string, cfg PinConfig) error {
	if _, ok := PinByID(id); !ok {
		return fmt.Errorf("unknown pin %q", id)
	}
	normalizeConfig(&cfg)
	if err := validateConfig(cfg); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Pins == nil {
		m.state.Pins = map[string]storedPin{}
	}
	if err := m.applyLocked(id, cfg, "set"); err != nil {
		ps := m.runtime[id]
		ps.lastError = err.Error()
		m.runtime[id] = ps
		return err
	}
	sp := m.state.Pins[id]
	sp.Current = cfg
	m.state.Pins[id] = sp
	if err := m.store.Save(m.state); err != nil {
		log.Printf("gpio: %s: saved state write failed: %v", id, err)
		return err
	}
	ps := m.runtime[id]
	ps.configured = true
	ps.config = cfg
	ps.lastError = ""
	m.runtime[id] = ps
	return nil
}

func (m *Manager) SaveDefault(id string) error {
	if _, ok := PinByID(id); !ok {
		return fmt.Errorf("unknown pin %q", id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Pins == nil {
		m.state.Pins = map[string]storedPin{}
	}
	ps, ok := m.runtime[id]
	if !ok || !ps.configured {
		return fmt.Errorf("pin not configured")
	}
	def := cloneConfig(&ps.config)
	ps.defaultCfg = def
	m.runtime[id] = ps
	sp := m.state.Pins[id]
	sp.Default = def
	sp.Current = ps.config
	m.state.Pins[id] = sp
	if err := m.store.Save(m.state); err != nil {
		return err
	}
	defPin, _ := PinByID(id)
	log.Printf("gpio: %s (%s): default snapshot saved (%s)", defPin.Label(), id, describeConfig(ps.config))
	return nil
}

func (m *Manager) ResetToDefault(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.runtime[id]
	if !ok || ps.defaultCfg == nil {
		return fmt.Errorf("no default saved")
	}
	cfg := *ps.defaultCfg
	if err := m.applyLocked(id, cfg, "reset-default"); err != nil {
		ps.lastError = err.Error()
		m.runtime[id] = ps
		return err
	}
	sp := m.state.Pins[id]
	sp.Current = cfg
	m.state.Pins[id] = sp
	ps.config = cfg
	ps.configured = true
	ps.lastError = ""
	m.runtime[id] = ps
	return m.store.Save(m.state)
}

func (m *Manager) applyLocked(id string, cfg PinConfig, source string) (err error) {
	def, ok := PinByID(id)
	if !ok {
		return fmt.Errorf("unknown pin")
	}
	linuxGPIO := def.LinuxGPIO()

	log.Printf("gpio: [%s] %s (%s, linux %d): mode=%s, %s",
		source, def.Label(), id, linuxGPIO, cfg.Mode, describeConfig(cfg))
	if def.MuxNote != "" {
		log.Printf("gpio: [%s] %s: mux hint: %s", source, id, def.MuxNote)
	}
	if m.backend.Hardware() {
		ensurePinMuxGPIO(def)
	}

	defer func() {
		ps := m.runtime[id]
		if err != nil {
			log.Printf("gpio: [%s] %s (%s): failed: %v", source, def.Label(), id, err)
			return
		}
		msg := fmt.Sprintf("gpio: [%s] %s (%s): ok", source, def.Label(), id)
		if ps.pwmBackend != "" {
			msg += fmt.Sprintf(", pwm=%s", ps.pwmBackend)
		}
		if cfg.Mode == ModeOut && cfg.Output == OutputDigital {
			msg += fmt.Sprintf(", level=%t", cfg.Value)
		}
		log.Print(msg)
	}()

	m.sw.stop(id)
	if def.HardwarePWM != nil {
		if err := m.backend.StopHardwarePWM(def.HardwarePWM); err != nil {
			log.Printf("gpio: [%s] %s: release hardware PWM: %v", source, id, err)
		}
	}

	ps := m.runtime[id]
	ps.pwmBackend = ""
	if !m.backend.Hardware() && cfg.Mode == ModeOut && cfg.Output == OutputPWM {
		ps.pwmBackend = PWMBackendStub
	}

	if cfg.Mode == ModeIn {
		if err := m.backend.Export(linuxGPIO); err != nil {
			return err
		}
		if err := m.backend.SetDirection(linuxGPIO, ModeIn); err != nil {
			return err
		}
		m.runtime[id] = ps
		return nil
	}

	// output
	if cfg.Output == OutputPWM {
		useHW := def.HardwarePWM != nil && m.backend.HardwarePWMEnabled(def.HardwarePWM)
		if useHW {
			_ = m.backend.Unexport(linuxGPIO)
			if err := m.backend.SetHardwarePWM(def.HardwarePWM, cfg.FrequencyHz, cfg.DutyPercent); err != nil {
				useHW = false
			} else {
				ps.pwmBackend = PWMBackendHardware
				m.runtime[id] = ps
				return nil
			}
		}
		if err := m.backend.Export(linuxGPIO); err != nil {
			return err
		}
		if err := m.backend.SetDirection(linuxGPIO, ModeOut); err != nil {
			return err
		}
		if m.backend.Hardware() {
			ps.pwmBackend = PWMBackendSoftware
		} else {
			ps.pwmBackend = PWMBackendStub
		}
		m.runtime[id] = ps
		m.sw.start(id, linuxGPIO, cfg.FrequencyHz, cfg.DutyPercent)
		return nil
	}

	// digital out
	// GPIO2 is not registered on this kernel (no ff540000.gpio). linux GPIO 72
	// would be aimed at a different gpiochip. Drive the bank registers directly.
	if m.backend.Hardware() && def.Bank == 2 && !gpioBankProbed(2) {
		detail, match, ferr := forceGPIOLevel(def, cfg.Value)
		if ferr != nil && !errors.Is(ferr, errMMIOBlind) {
			return fmt.Errorf("регистры GPIO/IOMUX: %w", ferr)
		}
		if !match {
			return fmt.Errorf("площадка не приняла уровень %t (%s)", cfg.Value, detail)
		}
		m.runtime[id] = ps
		return nil
	}
	if err := m.backend.Export(linuxGPIO); err != nil {
		return err
	}
	if err := m.backend.SetDirection(linuxGPIO, ModeOut); err != nil {
		return err
	}
	if err := m.backend.Write(linuxGPIO, cfg.Value); err != nil {
		return err
	}
	if m.backend.Hardware() {
		detail, match, ferr := forceGPIOLevel(def, cfg.Value)
		if errors.Is(ferr, errMMIOBlind) {
			if werr := m.backend.Write(linuxGPIO, cfg.Value); werr != nil {
				return werr
			}
			rb, rerr := m.backend.ReadLevel(linuxGPIO)
			if rerr != nil || rb != cfg.Value {
				return fmt.Errorf("уровень %t не подтверждён gpio-cdev (read=%v err=%v); %s", cfg.Value, rb, rerr, detail)
			}
		} else if ferr != nil {
			return fmt.Errorf("регистры GPIO/IOMUX: %w", ferr)
		} else if !match {
			return fmt.Errorf("площадка не приняла уровень %t (%s)", cfg.Value, detail)
		}
	}
	m.runtime[id] = ps
	return nil
}

func (m *Manager) readLevel(def PinDef) (bool, error) {
	if err := m.backend.Export(def.LinuxGPIO()); err != nil {
		// may already be exported
	}
	_ = m.backend.SetDirection(def.LinuxGPIO(), ModeIn)
	return m.backend.Read(def.LinuxGPIO())
}

func normalizeConfig(cfg *PinConfig) {
	if cfg.Mode == ModeIn {
		cfg.Output = ""
		return
	}
	if cfg.Output == "" {
		cfg.Output = OutputDigital
	}
	if cfg.Output == OutputPWM && cfg.FrequencyHz <= 0 {
		cfg.FrequencyHz = 1000
	}
}

func validateConfig(cfg PinConfig) error {
	if cfg.Mode != ModeIn && cfg.Mode != ModeOut {
		return fmt.Errorf("invalid mode")
	}
	if cfg.Mode == ModeOut {
		if cfg.Output != OutputDigital && cfg.Output != OutputPWM {
			return fmt.Errorf("invalid output kind")
		}
		if cfg.Output == OutputPWM {
			if cfg.DutyPercent < 0 || cfg.DutyPercent > 100 {
				return fmt.Errorf("duty must be 0-100")
			}
		}
	}
	return nil
}

func cloneConfig(c *PinConfig) *PinConfig {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

func describeConfig(cfg PinConfig) string {
	switch cfg.Mode {
	case ModeIn:
		return "input"
	case ModeOut:
		if cfg.Output == OutputPWM {
			return fmt.Sprintf("output pwm %.0f Hz, duty %.0f%%", cfg.FrequencyHz, cfg.DutyPercent)
		}
		return fmt.Sprintf("output digital, level=%t", cfg.Value)
	default:
		return "unknown"
	}
}
