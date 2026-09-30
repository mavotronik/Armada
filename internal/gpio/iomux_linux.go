//go:build linux

package gpio

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	iomuxDevDefault  = "/dev/iomux"
	iomuxMuxGPIO     = 0
	iomuxIOCDataSize = 12 // three uint32 fields in iomux_ioctl_data
)

var (
	iomuxDevPath  = iomuxDevDefault
	iomuxWarnOnce sync.Once
)

// SetIomuxDevice overrides the default /dev/iomux path (empty disables auto pinmux).
func SetIomuxDevice(path string) {
	iomuxDevPath = path
}

func init() {
	lookupIOCBase = func() uint64 {
		if b, ok := dtRegByCompatible("rockchip,rv1106-ioc"); ok {
			return b
		}
		if b, ok := dtRegByCompatible("rockchip,rv1103-ioc"); ok {
			return b
		}
		return rv1106IOCBase
	}
}

func ensurePinMuxGPIO(def PinDef) {
	if iomuxDevPath == "" {
		return
	}
	pinIdx := def.MuxPinIndex()
	_ = tryIomuxDevice(def, pinIdx)
	if err := setRV1106MuxMem(def.Bank, pinIdx, iomuxMuxGPIO); err != nil {
		iomuxWarnOnce.Do(func() {
			log.Printf("gpio: pinmux: запись IOC не удалась: %v", err)
		})
	}
}

// forceGPIOLevel sets mux to GPIO and drives the RV1106 GPIO data/direction registers.
// The returned detail is mux/DR/DDR/EXT. ok is whether the pad (EXT) matches high.
func forceGPIOLevel(def PinDef, high bool) (detail string, ok bool, err error) {
	pin := def.MuxPinIndex()
	iomuxNote := applyKernelIomux(def, pin)
	base, src := gpioControllerBase(def.Bank, def.LinuxGPIO())
	if base == 0 {
		return "", false, fmt.Errorf("нет адреса GPIO bank %d; %s", def.Bank, iomuxNote)
	}
	var clkNote string
	if def.Bank == 2 {
		clkNote = enableRV1106GPIO2()
	}
	// pread/pwrite cannot reach SoC registers above RAM on ARM (EFAULT). mmap can.
	muxErr := setRV1106MuxMem(def.Bank, pin, iomuxMuxGPIO)
	snap, drvErr := driveRV1106GPIO(base, pin, high, 0)
	mux, _ := readRV1106Mux(def.Bank, pin)
	want := 0
	if high {
		want = 1
	}
	detail = fmt.Sprintf("base=%#x (%s) pin=%d mux=%d ver=%#08x r00=%#08x r04=%#08x r08=%#08x r70=%#08x EXT=%d clk=[%s] iomux=[%s] muxErr=%v drvErr=%v",
		base, src, pin, mux, snap.ver, snap.rawDR, snap.rawDDR04, snap.rawDDR, snap.rawEXT, snap.ext, clkNote, iomuxNote, muxErr, drvErr)
	log.Printf("gpio: %s: %s", def.ID, detail)
	if snap.ver == 0 {
		return detail, false, fmt.Errorf("%w: %s", errMMIOBlind, detail)
	}
	return detail, snap.ext == want, nil
}

// enableRV1106GPIO2 ungates pclk/dbclk for GPIO2. The bank is absent from
// this kernel's device tree, so the driver never enables those clocks and
// the block at 0xff540000 reads as zero. Parent pclk_vo_root is critical.
// Gate bits are CLK_GATE_SET_TO_DISABLE | HIWORD_MASK: write 0 with the mask to enable.
func enableRV1106GPIO2() string {
	const cru = 0xff3a0000
	const gate = cru + 0x1c80c // RV1106_VOCLKGATE_CON(3)
	before, _ := readPhys32(gate)
	if err := writePhys32(gate, 1<<16); err != nil { // pclk_gpio2 bit 0
		return err.Error()
	}
	if err := writePhys32(gate, 1<<17); err != nil { // dbclk_gpio2 bit 1
		return err.Error()
	}
	after, _ := readPhys32(gate)
	return fmt.Sprintf("gate %#x %08x→%08x", uint64(gate), before, after)
}

func gpioBankProbed(bank int) bool {
	base, ok := rv1106GPIOBase[bank]
	if !ok {
		return false
	}
	want := fmt.Sprintf("%x.gpio", base)
	entries, err := os.ReadDir("/sys/bus/platform/devices")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), want) {
			return true
		}
	}
	return false
}

func applyKernelIomux(def PinDef, pin int) string {
	ensureDevIomux()
	if iomuxDevPath == "" {
		return "disabled"
	}
	if _, err := os.Stat(iomuxDevPath); err != nil {
		return "no /dev/iomux and no /sys/class/misc/iomux"
	}
	mux, err := iomuxGet(def.Bank, pin)
	if err != nil {
		return "get " + err.Error()
	}
	if mux != iomuxMuxGPIO {
		if err := iomuxSet(def.Bank, pin, iomuxMuxGPIO); err != nil {
			return fmt.Sprintf("mux=%d set %v", mux, err)
		}
		return fmt.Sprintf("mux %d→0", mux)
	}
	return "mux=0"
}

func ensureDevIomux() {
	if iomuxDevPath == "" {
		return
	}
	if _, err := os.Stat(iomuxDevPath); err == nil {
		return
	}
	raw, err := os.ReadFile("/sys/class/misc/iomux/dev")
	if err != nil {
		return
	}
	parts := strings.Split(strings.TrimSpace(string(raw)), ":")
	if len(parts) != 2 {
		return
	}
	maj, err1 := strconv.ParseUint(parts[0], 10, 32)
	min, err2 := strconv.ParseUint(parts[1], 10, 32)
	if err1 != nil || err2 != nil {
		return
	}
	dev := int(unix.Mkdev(uint32(maj), uint32(min)))
	if err := unix.Mknod(iomuxDevPath, unix.S_IFCHR|0600, dev); err != nil {
		log.Printf("gpio: mknod %s: %v", iomuxDevPath, err)
		return
	}
	log.Printf("gpio: created %s (%d:%d)", iomuxDevPath, maj, min)
}

func tryIomuxDevice(def PinDef, pinIdx int) bool {
	if _, err := os.Stat(iomuxDevPath); err != nil {
		return false
	}
	mux, err := iomuxGet(def.Bank, pinIdx)
	if err != nil {
		log.Printf("gpio: pinmux: %s bank=%d pin=%d get: %v", def.ID, def.Bank, pinIdx, err)
		return false
	}
	if mux == iomuxMuxGPIO {
		return true
	}
	if err := iomuxSet(def.Bank, pinIdx, iomuxMuxGPIO); err != nil {
		log.Printf("gpio: pinmux: %s bank=%d pin=%d set GPIO: %v", def.ID, def.Bank, pinIdx, err)
		return false
	}
	log.Printf("gpio: pinmux: %s bank=%d pin=%d mux %d → %d (GPIO)", def.ID, def.Bank, pinIdx, mux, iomuxMuxGPIO)
	return true
}

var rv1106GPIOBase = map[int]uint64{
	0: 0xff380000,
	1: 0xff530000,
	2: 0xff540000,
	3: 0xff550000,
	4: 0xff560000,
}

func setRV1106MuxMem(bank, pin, mux int) error {
	if !socIsRV1106() {
		return fmt.Errorf("не RV1103/RV1106 (%s)", socID())
	}
	phys, bit, ok := RV1106MuxReg(bank, pin)
	if !ok {
		return fmt.Errorf("нет IOMUX для bank=%d pin=%d", bank, pin)
	}
	return writePhys32(phys, RV1106MuxWriteValue(bit, mux))
}

func readRV1106Mux(bank, pin int) (int, error) {
	phys, bit, ok := RV1106MuxReg(bank, pin)
	if !ok {
		return -1, fmt.Errorf("нет IOMUX")
	}
	v, err := readPhys32(phys)
	if err != nil {
		return -1, err
	}
	return int((v >> uint(bit)) & 0xf), nil
}

type gpioSnap struct {
	ver, rawDR, rawDDR, rawDDR04, rawEXT, rawEXT50 uint32
	dr, ddr, ext                                   int
}

func gpioControllerBase(bank, linuxGPIO int) (uint64, string) {
	if b, src, ok := sysfsGPIOPhys(linuxGPIO); ok {
		return b, src
	}
	if b, ok := dtAliasReg(fmt.Sprintf("gpio%d", bank)); ok {
		return b, "alias gpio" + strconv.Itoa(bank)
	}
	if b, ok := rv1106GPIOBase[bank]; ok {
		return b, "builtin"
	}
	return 0, ""
}

func sysfsGPIOPhys(linuxGPIO int) (uint64, string, bool) {
	const dir = "/sys/class/gpio"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, "", false
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "gpiochip") {
			continue
		}
		base, err := strconv.Atoi(strings.TrimPrefix(name, "gpiochip"))
		if err != nil {
			continue
		}
		ngpio := readSysfsInt(filepath.Join(dir, name, "ngpio"), 32)
		if linuxGPIO < base || linuxGPIO >= base+ngpio {
			continue
		}
		devLink, _ := os.Readlink(filepath.Join(dir, name, "device"))
		uevent, _ := os.ReadFile(filepath.Join(dir, name, "device", "uevent"))
		text := devLink + "\n" + string(uevent)
		if phys, ok := physFromGPIOText(text); ok {
			return phys, name + " " + devLink, true
		}
		return 0, name + " (no addr in " + devLink + ")", false
	}
	return 0, "", false
}

func readSysfsInt(path string, fallback int) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return fallback
	}
	return n
}

func physFromGPIOText(text string) (uint64, bool) {
	lower := strings.ToLower(text)
	for _, key := range []string{"gpio@", ".gpio"} {
		idx := strings.Index(lower, key)
		if idx < 0 {
			continue
		}
		var hex string
		if key == "gpio@" {
			hex = hexPrefix(lower[idx+len(key):])
		} else {
			start := idx
			for start > 0 && isHex(lower[start-1]) {
				start--
			}
			hex = lower[start:idx]
		}
		if hex == "" {
			continue
		}
		v, err := strconv.ParseUint(hex, 16, 64)
		if err == nil && v != 0 {
			return v, true
		}
	}
	return 0, false
}

func hexPrefix(s string) string {
	i := 0
	for i < len(s) && isHex(s[i]) {
		i++
	}
	return s[:i]
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f')
}

func probeGPIORegs() string {
	entries, err := os.ReadDir("/sys/bus/platform/devices")
	if err != nil {
		return err.Error()
	}
	var parts []string
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if !strings.Contains(name, "gpio") {
			continue
		}
		phys, ok := physFromGPIOText(name)
		if !ok {
			continue
		}
		v, err := readPhys32(phys + 0x78)
		if err != nil {
			parts = append(parts, e.Name()+"="+err.Error())
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%#08x", e.Name(), v))
	}
	if len(parts) == 0 {
		return "no gpio platform device"
	}
	return strings.Join(parts, " ")
}

func dtAliasReg(alias string) (uint64, bool) {
	raw, err := os.ReadFile("/proc/device-tree/aliases/" + alias)
	if err != nil {
		return 0, false
	}
	rel := strings.TrimRight(string(raw), "\x00")
	if rel == "" {
		return 0, false
	}
	return readBEReg("/proc/device-tree" + rel + "/reg")
}

func dtRegByCompatible(substr string) (uint64, bool) {
	var found uint64
	_ = filepath.Walk("/proc/device-tree", func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || info.Name() != "compatible" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(b), substr) {
			return nil
		}
		reg, ok := readBEReg(filepath.Join(filepath.Dir(path), "reg"))
		if ok && reg != 0 {
			found = reg
			return filepath.SkipAll
		}
		return nil
	})
	return found, found != 0
}

func readBEReg(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 4 {
		return 0, false
	}
	addr := uint64(binary.BigEndian.Uint32(b[:4]))
	if len(b) >= 8 && addr == 0 {
		addr = uint64(binary.BigEndian.Uint32(b[4:8]))
	}
	return addr, addr != 0
}

func driveRV1106GPIO(base uint64, pin int, high bool, ver uint32) (gpioSnap, error) {
	// RV1106 is GPIO v2. A v1 read-modify-write has no hiword mask, so the block ignores it.
	if err := writeGPIOV2Bit(base, 0x08, pin, true); err != nil {
		return gpioSnap{}, err
	}
	if err := writeGPIOV2Bit(base, 0x00, pin, high); err != nil {
		return gpioSnap{}, err
	}
	return readGPIOV2Snap(base, pin, ver)
}

func readGPIOV2Snap(base uint64, pin int, ver uint32) (gpioSnap, error) {
	drReg, drBit := splitV2(base, 0x00, pin)
	ddrReg, _ := splitV2(base, 0x08, pin)
	rawDR, err := readPhys32(drReg)
	if err != nil {
		return gpioSnap{}, err
	}
	rawDDR, err := readPhys32(ddrReg)
	if err != nil {
		return gpioSnap{}, err
	}
	rawDDR04, err := readPhys32(base + 0x04)
	if err != nil {
		return gpioSnap{}, err
	}
	rawEXT, err := readPhys32(base + 0x70)
	if err != nil {
		return gpioSnap{}, err
	}
	rawEXT50, err := readPhys32(base + 0x50)
	if err != nil {
		return gpioSnap{}, err
	}
	if ver == 0 {
		ver, _ = readPhys32(base + 0x78)
	}
	bit := func(v uint32, b int) int {
		if v&(1<<uint(b)) != 0 {
			return 1
		}
		return 0
	}
	return gpioSnap{
		ver:      ver,
		rawDR:    rawDR,
		rawDDR:   rawDDR,
		rawDDR04: rawDDR04,
		rawEXT:   rawEXT,
		rawEXT50: rawEXT50,
		dr:       bit(rawDR, drBit),
		ddr:      bit(rawDDR, drBit),
		ext:      int((rawEXT >> uint(pin)) & 1),
	}, nil
}

func writeGPIOV2Bit(base uint64, regOff, bit int, high bool) error {
	reg, b := splitV2(base, regOff, bit)
	var data uint32
	if high {
		data = uint32(1<<uint(b)) | uint32(1<<uint(b+16))
	} else {
		data = uint32(1 << uint(b+16))
	}
	return writePhys32(reg, data)
}

func splitV2(base uint64, regOff, bit int) (uint64, int) {
	reg := base + uint64(regOff)
	if bit >= 16 {
		reg += 4
		bit -= 16
	}
	return reg, bit
}

func socIsRV1106() bool {
	for _, path := range []string{"/proc/device-tree/compatible", "/proc/device-tree/model"} {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		s := strings.ToLower(strings.ReplaceAll(string(b), "\x00", " "))
		if strings.Contains(s, "rv1106") || strings.Contains(s, "rv1103") || strings.Contains(s, "luckfox") {
			return true
		}
	}
	return false
}

func socID() string {
	b, err := os.ReadFile("/proc/device-tree/compatible")
	if err != nil {
		return err.Error()
	}
	return strings.ReplaceAll(string(b), "\x00", " ")
}

func writePhys32(phys uint64, val uint32) error {
	_, err := accessPhys32(phys, true, val)
	return err
}

func readPhys32(phys uint64) (uint32, error) {
	return accessPhys32(phys, false, 0)
}

func accessPhys32(phys uint64, write bool, val uint32) (uint32, error) {
	const page = 4096
	pageBase := phys &^ (page - 1)
	off := int(phys - pageBase)
	f, err := os.OpenFile("/dev/mem", os.O_RDWR|os.O_SYNC, 0)
	if err != nil {
		return 0, fmt.Errorf("/dev/mem: %w", err)
	}
	defer f.Close()
	mem, err := unix.Mmap(int(f.Fd()), int64(pageBase), page, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return 0, fmt.Errorf("mmap %#x: %w", phys, err)
	}
	defer unix.Munmap(mem)
	ptr := unsafe.Add(unsafe.Pointer(&mem[0]), off)
	if write {
		*(*uint32)(ptr) = val
	}
	return *(*uint32)(ptr), nil
}

func iomuxGet(bank, pin int) (uint32, error) {
	fd, err := unix.Open(iomuxDevPath, unix.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer unix.Close(fd)
	return iomuxIO(fd, iomuxIOCMuxGet(), bank, pin, 0)
}

func iomuxSet(bank, pin int, mux uint32) error {
	fd, err := unix.Open(iomuxDevPath, unix.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	_, err = iomuxIO(fd, iomuxIOCMuxSet(), bank, pin, mux)
	return err
}

func iomuxIOCMuxSet() uintptr { return iomuxIOWR('P', 0, iomuxIOCDataSize) }
func iomuxIOCMuxGet() uintptr { return iomuxIOWR('P', 1, iomuxIOCDataSize) }

func iomuxIOWR(typ, nr, size byte) uintptr {
	const iocReadWrite = 3
	const dirShift, typeShift, nrShift, sizeShift = 30, 8, 0, 16
	return (iocReadWrite << dirShift) | (uintptr(typ) << typeShift) | uintptr(nr) | (uintptr(size) << sizeShift)
}

func iomuxIO(fd int, req uintptr, bank, pin int, mux uint32) (uint32, error) {
	data := iomuxData{bank: uint32(bank), pin: uint32(pin), mux: mux}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(&data)))
	if errno != 0 {
		return 0, fmt.Errorf("ioctl: %w", errno)
	}
	return data.mux, nil
}

type iomuxData struct {
	bank, pin, mux uint32
}
