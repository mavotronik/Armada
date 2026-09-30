package gpio

import "errors"

// errMMIOBlind means /dev/mem did not reach the GPIO controller.
var errMMIOBlind = errors.New("mmio blind")

// RV1106 IOC (pin controller) base. Pinctrl uses this syscon, not the GRF at 0xff000000.
const rv1106IOCBase uint64 = 0xff538000

// rv1106GroupOffset is the IOMUX register offset for each bank group (A–D).
// -1 means the group is not mapped (do not write offset 0).
var rv1106GroupOffset = map[int][4]int32{
	1: {0x0, 0x08, 0x10, 0x18},
	2: {0x10020, 0x10028, -1, -1},
	3: {0x20040, 0x20048, 0x20050, 0x20058},
	4: {0x30000, 0x30008, 0x30010, -1},
}

// RV1106MuxReg returns the physical IOMUX address and bit shift for a bank pin (0–31).
// Mux value 0 is GPIO. The write uses the Rockchip hiword mask (bits bit+16).
func RV1106MuxReg(bank, pin int) (phys uint64, bit int, ok bool) {
	if pin < 0 || pin > 31 {
		return 0, 0, false
	}
	offs, known := rv1106GroupOffset[bank]
	if !known {
		return 0, 0, false
	}
	group := pin / 8
	base := offs[group]
	if base < 0 {
		return 0, 0, false
	}
	reg := uint64(base)
	if pin%8 >= 4 {
		reg += 4
	}
	bit = (pin % 4) * 4
	return lookupIOCBase() + reg, bit, true
}

// lookupIOCBase is replaced on Linux with the address from the live device tree.
var lookupIOCBase = func() uint64 { return rv1106IOCBase }

// RV1106MuxWriteValue is the 32-bit store for a 4-bit mux field (hiword enable).
func RV1106MuxWriteValue(bit, mux int) uint32 {
	mask := uint32(0xf)
	return (mask << uint(bit+16)) | ((uint32(mux) & mask) << uint(bit))
}

// PinMuxPinIndex is the bank-local pin index for Rockchip /dev/iomux (group*8 + index).
func PinMuxPinIndex(bank int, group rune, index int) int {
	g := map[rune]int{'A': 0, 'B': 1, 'C': 2, 'D': 3}[group]
	return g*8 + index
}

func (p PinDef) MuxPinIndex() int {
	if p.Bank == 0 && p.Header == 0 {
		return 0
	}
	return PinMuxPinIndex(p.Bank, p.Group, p.Index)
}
