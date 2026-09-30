package gpio

import "testing"

func TestLinuxNumGPIO1_C7(t *testing.T) {
	got := LinuxNum(1, 'C', 7)
	if got != 55 {
		t.Fatalf("GPIO1_C7 = %d, want 55", got)
	}
}

func TestRV1106MuxRegGPIO2B(t *testing.T) {
	// GPIO2_B0 = bank pin 8, GPIO2_B1 = pin 9. IOC 0xff538000 + 0x10028.
	phys, bit, ok := RV1106MuxReg(2, 8)
	if !ok || phys != 0xff548028 || bit != 0 {
		t.Fatalf("GPIO2_B0: phys=%#x bit=%d ok=%v", phys, bit, ok)
	}
	if got := RV1106MuxWriteValue(bit, 0); got != 0x000f0000 {
		t.Fatalf("write value B0 = %#x", got)
	}
	phys, bit, ok = RV1106MuxReg(2, 9)
	if !ok || phys != 0xff548028 || bit != 4 {
		t.Fatalf("GPIO2_B1: phys=%#x bit=%d ok=%v", phys, bit, ok)
	}
	if got := RV1106MuxWriteValue(bit, 0); got != 0x00f00000 {
		t.Fatalf("write value B1 = %#x", got)
	}
}

func TestPinMuxPinIndex(t *testing.T) {
	if got := PinMuxPinIndex(1, 'C', 7); got != 23 {
		t.Fatalf("GPIO1_C7 mux pin = %d, want 23", got)
	}
	p, _ := PinByID("24")
	if p.MuxPinIndex() != 0 {
		t.Fatalf("GPIO2_A0 mux pin = %d, want 0", p.MuxPinIndex())
	}
}

func TestCatalogExcludesUART2ConsolePins(t *testing.T) {
	for _, id := range []string{"1", "2"} {
		if _, ok := PinByID(id); ok {
			t.Fatalf("pin %s must not be in catalog", id)
		}
	}
}

func TestCatalogUARTWarnings(t *testing.T) {
	uart4 := map[string]string{"4": "CTS", "5": "RTS", "6": "TX", "7": "RX"}
	for id, line := range uart4 {
		p, ok := PinByID(id)
		if !ok {
			t.Fatalf("missing pin %s", id)
		}
		if p.UART != "UART4" || p.UARTLine != line {
			t.Fatalf("pin %s: got %s %s", id, p.UART, p.UARTLine)
		}
	}
	for _, id := range []string{"19", "20"} {
		p, ok := PinByID(id)
		if !ok {
			t.Fatalf("missing pin %s", id)
		}
		if p.UART != "UART3" {
			t.Fatalf("pin %s: want UART3", id)
		}
	}
}

func TestLEDInCatalog(t *testing.T) {
	p, ok := PinByID("led")
	if !ok {
		t.Fatal("missing led")
	}
	if p.LinuxGPIO() != 118 {
		t.Fatalf("led gpio = %d, want 118", p.LinuxGPIO())
	}
}
