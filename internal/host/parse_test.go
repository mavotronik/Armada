package host

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProcStatCPU(t *testing.T) {
	data := []byte(`cpu  100 200 300 400 500 0 0 0 0 0
cpu0 50 100 150 200 250 0 0 0 0 0
`)
	cur, err := parseProcStatCPU(data)
	if err != nil {
		t.Fatal(err)
	}
	prev := cpuTimes{idle: 400, total: 1500}
	pct := cpuUsagePercent(prev, cur)
	if pct < 0 || pct > 100 {
		t.Fatalf("unexpected cpu pct: %v", pct)
	}
}

func TestParseLoadAvg(t *testing.T) {
	l1, l5, l15, err := parseLoadAvg([]byte("0.42 0.38 0.35 1/234 5678\n"))
	if err != nil {
		t.Fatal(err)
	}
	if l1 != 0.42 || l5 != 0.38 || l15 != 0.35 {
		t.Fatalf("load: %v %v %v", l1, l5, l15)
	}
}

func TestParseMeminfo(t *testing.T) {
	data := []byte(`MemTotal:       256000 kB
MemAvailable:   128000 kB
`)
	total, avail, err := parseMeminfo(data)
	if err != nil {
		t.Fatal(err)
	}
	if total != 256000*1024 || avail != 128000*1024 {
		t.Fatalf("mem: total=%d avail=%d", total, avail)
	}
}

func TestParseNetDev(t *testing.T) {
	data := []byte(`Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1000       10    0    0    0     0          0         0     1000       10    0    0    0     0       0          0
  eth0: 5000       50    0    0    0     0          0         0     3000       30    0    0    0     0       0          0
`)
	list, err := parseNetDev(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].name != "eth0" {
		t.Fatalf("net: %+v", list)
	}
	if list[0].rxBytes != 5000 || list[0].txBytes != 3000 {
		t.Fatalf("bytes: %+v", list[0])
	}
}

func TestCollectorFromFixtures(t *testing.T) {
	root := filepath.Join("testdata", "proc")
	sys := filepath.Join("testdata", "sys")
	if _, err := os.Stat(root); err != nil {
		t.Skip("fixtures missing")
	}
	c := NewCollectorFromRoots(root, sys)
	c.tick()
	s := c.Snapshot()
	if s.CPU.Load1 != 0.15 {
		t.Fatalf("load1: %v", s.CPU.Load1)
	}
	if s.RAM.TotalBytes == 0 {
		t.Fatal("ram total zero")
	}
	if s.Uptime != 3600 {
		t.Fatalf("uptime: %d", s.Uptime)
	}
	if s.TemperatureC == nil || *s.TemperatureC != 42.5 {
		t.Fatalf("temp: %v", s.TemperatureC)
	}
}
