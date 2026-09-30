package gpio

// gpioChipBaseFromName maps a gpiochip device suffix to the Linux GPIO base.
// Rockchip uses either bank index (gpiochip0..4 → base 0,32,…) or base in the name (gpiochip32, …).
func gpioChipBaseFromName(chipSuffix int) int {
	if chipSuffix < 32 {
		return chipSuffix * 32
	}
	return chipSuffix
}

// BankOffset splits a global Linux GPIO number into bank line offset (0–31).
func BankOffset(global int) (bank, offset int) {
	return global / 32, global % 32
}
