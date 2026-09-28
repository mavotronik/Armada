package host

import (
	"net"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Collector polls host metrics and keeps the latest snapshot.
type Collector struct {
	procRoot string
	sysRoot  string

	mu       sync.RWMutex
	snapshot Snapshot

	prevCPU  cpuTimes
	prevNet  map[string]netCounters
	prevNetAt time.Time
	hasCPU   bool
}

func NewCollector() *Collector {
	return &Collector{
		procRoot: "/proc",
		sysRoot:  "/sys",
		prevNet:  make(map[string]netCounters),
	}
}

// NewCollectorFromRoots is for tests with fixture trees.
func NewCollectorFromRoots(procRoot, sysRoot string) *Collector {
	return &Collector{
		procRoot: procRoot,
		sysRoot:  sysRoot,
		prevNet:  make(map[string]netCounters),
	}
}

func (c *Collector) procPath(parts ...string) string {
	return filepath.Join(append([]string{c.procRoot}, parts...)...)
}

func (c *Collector) Start(interval time.Duration) {
	c.tick()
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			c.tick()
		}
	}()
}

func (c *Collector) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot
}

func (c *Collector) tick() {
	now := time.Now()

	statData, _ := readFile(c.procPath("stat"))
	loadData, _ := readFile(c.procPath("loadavg"))
	upData, _ := readFile(c.procPath("uptime"))
	memData, _ := readFile(c.procPath("meminfo"))
	netData, _ := readFile(c.procPath("net", "dev"))

	var cpuPct float64
	var l1, l5, l15 float64
	if cur, err := parseProcStatCPU(statData); err == nil {
		if c.hasCPU {
			cpuPct = cpuUsagePercent(c.prevCPU, cur)
		}
		c.prevCPU = cur
		c.hasCPU = true
	}
	if loadData != nil {
		l1, l5, l15, _ = parseLoadAvg(loadData)
	}

	var uptime int64
	if upData != nil {
		uptime, _ = parseUptimeSeconds(upData)
	}

	var ram RAMMetrics
	if memData != nil {
		total, avail, err := parseMeminfo(memData)
		if err == nil && total > 0 {
			used := total - avail
			if avail > total {
				used = total
			}
			ram = RAMMetrics{
				TotalBytes:   total,
				UsedBytes:    used,
				UsagePercent: float64(used) / float64(total) * 100,
			}
		}
	}

	disk := diskForPath("/")

	var nets []NetInterface
	var netList []netCounters
	if netData != nil {
		netList, _ = parseNetDev(netData)
	}
	elapsed := now.Sub(c.prevNetAt).Seconds()
	if c.prevNetAt.IsZero() {
		elapsed = 0
	}
	for _, nc := range netList {
		var rxBps, txBps float64
		if prev, ok := c.prevNet[nc.name]; ok && elapsed > 0 {
			if nc.rxBytes >= prev.rxBytes {
				rxBps = float64(nc.rxBytes-prev.rxBytes) / elapsed
			}
			if nc.txBytes >= prev.txBytes {
				txBps = float64(nc.txBytes-prev.txBytes) / elapsed
			}
		}
		c.prevNet[nc.name] = nc
		addrs := addrsForInterface(nc.name)
		nets = append(nets, NetInterface{
			Name:      nc.name,
			Addresses: addrs,
			RxBytes:   nc.rxBytes,
			TxBytes:   nc.txBytes,
			RxBps:     rxBps,
			TxBps:     txBps,
		})
	}
	c.prevNetAt = now

	hostname := readHostname()
	if c.procRoot != "/proc" {
		hostname = "test-node"
	}
	kernel := readKernelVersion()
	arch := readArch()
	temp := readThermal(c.sysRoot)

	primaryIP := primaryIPFromNets(nets)
	if primaryIP == "" {
		primaryIP = firstNonLoopbackIP()
	}

	snap := Snapshot{
		Timestamp:    now,
		Hostname:     hostname,
		Kernel:       kernel,
		Arch:         arch,
		Uptime:       uptime,
		CPU:          CPUMetrics{UsagePercent: cpuPct, Load1: l1, Load5: l5, Load15: l15},
		RAM:          ram,
		Disk:         disk,
		TemperatureC: temp,
		Network:      nets,
		Device: DeviceRow{
			Name:   hostname,
			Type:   "Node",
			IP:     primaryIP,
			Status: "online",
			Load:   cpuPct,
		},
	}

	c.mu.Lock()
	c.snapshot = snap
	c.mu.Unlock()
}

func diskForPath(path string) DiskMetrics {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return DiskMetrics{Path: path}
	}
	total := uint64(st.Blocks) * uint64(st.Bsize)
	free := uint64(st.Bavail) * uint64(st.Bsize)
	used := total - free
	var pct float64
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return DiskMetrics{
		Path:         path,
		TotalBytes:   total,
		UsedBytes:    used,
		UsagePercent: pct,
	}
}

func addrsForInterface(name string) []string {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		out = append(out, a.String())
	}
	return out
}

func primaryIPFromNets(nets []NetInterface) string {
	for _, n := range nets {
		for _, a := range n.Addresses {
			if stringsBeforeSlash(a) == "127.0.0.1" {
				continue
			}
			ip := stringsBeforeSlash(a)
			if ip != "" && !stringsHasPrefix(ip, "fe80:") {
				return ip
			}
		}
	}
	return ""
}

func stringsBeforeSlash(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return s[:i]
		}
	}
	return s
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
