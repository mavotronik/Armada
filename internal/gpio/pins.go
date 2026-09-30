package gpio

import "fmt"

// PWMDef describes optional hardware PWM on a header pin.
type PWMDef struct {
	Chip    int
	Channel int
}

// PinDef is a fixed Luckfox Pico Pro/Max controllable line.
type PinDef struct {
	ID        string
	Header    int // 0 for on-board LED
	Name      string
	Bank      int
	Group     rune
	Index     int
	Voltage   string // e.g. "1.8V"
	UART      string
	UARTLine  string
	MuxNote   string // peripheral that may hold pinmux until disabled in DT or via /dev/iomux
	HardwarePWM *PWMDef
}

// LinuxNum returns the Linux sysfs GPIO number.
func LinuxNum(bank int, group rune, index int) int {
	g := map[rune]int{'A': 0, 'B': 1, 'C': 2, 'D': 3}[group]
	return bank*32 + g*8 + index
}

func (p PinDef) LinuxGPIO() int {
	if p.Bank == 0 && p.Name == "" {
		return 0
	}
	return LinuxNum(p.Bank, p.Group, p.Index)
}

func (p PinDef) Label() string {
	if p.Header == 0 {
		return p.Name
	}
	return fmt.Sprintf("Pin %d · %s", p.Header, p.Name)
}

func pwm(chip int) *PWMDef {
	return &PWMDef{Chip: chip, Channel: 0}
}

// Catalog is the fixed set of manageable pins (UART2 console pins 1–2 excluded).
var Catalog = []PinDef{
	{ID: "4", Header: 4, Name: "GPIO1_C7_d", Bank: 1, Group: 'C', Index: 7, UART: "UART4", UARTLine: "CTS", HardwarePWM: pwm(11)},
	{ID: "5", Header: 5, Name: "GPIO1_C6_d", Bank: 1, Group: 'C', Index: 6, UART: "UART4", UARTLine: "RTS", HardwarePWM: pwm(10)},
	{ID: "6", Header: 6, Name: "GPIO1_C5_d", Bank: 1, Group: 'C', Index: 5, UART: "UART4", UARTLine: "TX", HardwarePWM: pwm(9)},
	{ID: "7", Header: 7, Name: "GPIO1_C4_d", Bank: 1, Group: 'C', Index: 4, UART: "UART4", UARTLine: "RX", HardwarePWM: pwm(8)},
	{ID: "9", Header: 9, Name: "GPIO1_D2_d", Bank: 1, Group: 'D', Index: 2, HardwarePWM: pwm(0)},
	{ID: "10", Header: 10, Name: "GPIO1_D3_d", Bank: 1, Group: 'D', Index: 3, HardwarePWM: pwm(11)},
	{ID: "11", Header: 11, Name: "GPIO2_B1_d", Bank: 2, Group: 'B', Index: 1},
	{ID: "12", Header: 12, Name: "GPIO1_C0_d", Bank: 1, Group: 'C', Index: 0, HardwarePWM: pwm(2), MuxNote: "SPI0 CS0 (M0) — при status=\"okay\" у &spi0 линия busy; для GPIO отключите SPI0 в device tree"},
	{ID: "14", Header: 14, Name: "GPIO1_C1_d", Bank: 1, Group: 'C', Index: 1, HardwarePWM: pwm(4), MuxNote: "SPI0 CLK (M0) — занят, пока включён &spi0 в device tree"},
	{ID: "15", Header: 15, Name: "GPIO1_C2_d", Bank: 1, Group: 'C', Index: 2, HardwarePWM: pwm(5), MuxNote: "SPI0 MOSI (M0) — занят, пока включён &spi0 в device tree"},
	{ID: "16", Header: 16, Name: "GPIO1_C3_d", Bank: 1, Group: 'C', Index: 3, HardwarePWM: pwm(6), MuxNote: "SPI0 MISO (M0) — занят, пока включён &spi0 в device tree"},
	{ID: "17", Header: 17, Name: "GPIO2_B0_d", Bank: 2, Group: 'B', Index: 0},
	{ID: "19", Header: 19, Name: "GPIO1_D0_d", Bank: 1, Group: 'D', Index: 0, UART: "UART3", UARTLine: "TX", HardwarePWM: pwm(3)},
	{ID: "20", Header: 20, Name: "GPIO1_D1_d", Bank: 1, Group: 'D', Index: 1, UART: "UART3", UARTLine: "RX", HardwarePWM: pwm(10)},
	{ID: "21", Header: 21, Name: "GPIO2_A4_d", Bank: 2, Group: 'A', Index: 4, MuxNote: "I2S0 (SDO0) — armada переключает pad в GPIO через /dev/mem (root), без пересборки DTB"},
	{ID: "22", Header: 22, Name: "GPIO2_A5_d", Bank: 2, Group: 'A', Index: 5, MuxNote: "I2S0 (SDI0) — armada переключает pad в GPIO через /dev/mem (root), без пересборки DTB"},
	{ID: "24", Header: 24, Name: "GPIO2_A0_d", Bank: 2, Group: 'A', Index: 0, MuxNote: "I2S0 (SCLK) — armada переключает pad в GPIO через /dev/mem (root), без пересборки DTB"},
	{ID: "25", Header: 25, Name: "GPIO2_A1_d", Bank: 2, Group: 'A', Index: 1, MuxNote: "I2S0 — armada переключает pad в GPIO через /dev/mem (root)"},
	{ID: "26", Header: 26, Name: "GPIO2_A2_d", Bank: 2, Group: 'A', Index: 2, MuxNote: "I2S0 (MCLK) — armada переключает pad в GPIO через /dev/mem (root)"},
	{ID: "27", Header: 27, Name: "GPIO2_A3_d", Bank: 2, Group: 'A', Index: 3, MuxNote: "I2S0 — armada переключает pad в GPIO через /dev/mem (root)"},
	{ID: "29", Header: 29, Name: "GPIO2_A6_d", Bank: 2, Group: 'A', Index: 6},
	{ID: "31", Header: 31, Name: "GPIO4_C0_z", Bank: 4, Group: 'C', Index: 0, Voltage: "1.8V"},
	{ID: "32", Header: 32, Name: "GPIO4_C1_z", Bank: 4, Group: 'C', Index: 1, Voltage: "1.8V"},
	{ID: "34", Header: 34, Name: "GPIO2_A7_d", Bank: 2, Group: 'A', Index: 7},
	{ID: "led", Header: 0, Name: "GPIO3_C6_d", Bank: 3, Group: 'C', Index: 6},
}

// PinByID returns catalog entry or false.
func PinByID(id string) (PinDef, bool) {
	for _, p := range Catalog {
		if p.ID == id {
			return p, true
		}
	}
	return PinDef{}, false
}
