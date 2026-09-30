package gpio

import "testing"

func TestGPIOChipBaseFromName(t *testing.T) {
	cases := map[int]int{
		0: 0, 1: 32, 2: 64, 3: 96, 4: 128,
		32: 32, 64: 64, 96: 96, 128: 128,
	}
	for suffix, want := range cases {
		if got := gpioChipBaseFromName(suffix); got != want {
			t.Fatalf("gpiochip%d base = %d, want %d", suffix, got, want)
		}
	}
}

func TestBankOffsetGPIO68(t *testing.T) {
	bank, off := BankOffset(68)
	if bank != 2 || off != 4 {
		t.Fatalf("GPIO68 bank=%d offset=%d, want 2/4", bank, off)
	}
}
