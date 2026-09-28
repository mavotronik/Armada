package host

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type cpuTimes struct {
	idle  uint64
	total uint64
}

func parseProcStatCPU(data []byte) (cpuTimes, error) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return cpuTimes{}, fmt.Errorf("short cpu line")
		}
		var nums []uint64
		for _, f := range fields[1:] {
			n, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				return cpuTimes{}, err
			}
			nums = append(nums, n)
		}
		var total uint64
		for _, n := range nums {
			total += n
		}
		idle := nums[3]
		if len(nums) > 4 {
			idle += nums[4]
		}
		return cpuTimes{idle: idle, total: total}, nil
	}
	return cpuTimes{}, fmt.Errorf("cpu line not found")
}

func cpuUsagePercent(prev, cur cpuTimes) float64 {
	dIdle := float64(cur.idle - prev.idle)
	dTotal := float64(cur.total - prev.total)
	if dTotal <= 0 {
		return 0
	}
	return (1 - dIdle/dTotal) * 100
}

func parseLoadAvg(data []byte) (l1, l5, l15 float64, err error) {
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("short loadavg")
	}
	l1, err = strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return
	}
	l5, err = strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return
	}
	l15, err = strconv.ParseFloat(fields[2], 64)
	return
}

func parseUptimeSeconds(data []byte) (int64, error) {
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("short uptime")
	}
	f, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return int64(f), nil
}

func parseMeminfo(data []byte) (total, available uint64, err error) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			total, err = parseMeminfoField(line)
			if err != nil {
				return
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			available, err = parseMeminfoField(line)
			if err != nil {
				return
			}
		}
	}
	if total == 0 {
		err = fmt.Errorf("MemTotal not found")
	}
	return
}

func parseMeminfoField(line string) (uint64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, fmt.Errorf("bad meminfo line")
	}
	n, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return n * 1024, nil
}

type netCounters struct {
	name    string
	rxBytes uint64
	txBytes uint64
}

func parseNetDev(data []byte) ([]netCounters, error) {
	var out []netCounters
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "Inter-") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		rx, err1 := strconv.ParseUint(fields[0], 10, 64)
		tx, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, netCounters{name: name, rxBytes: rx, txBytes: tx})
	}
	return out, nil
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func readThermal(root string) *float64 {
	glob := filepath.Join(root, "class/thermal/thermal_zone*/temp")
	if root != "/sys" {
		glob = filepath.Join(root, "thermal/thermal_zone*/temp")
	}
	matches, err := filepath.Glob(glob)
	if err != nil || len(matches) == 0 {
		return nil
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(data))
	milli, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	c := float64(milli) / 1000
	return &c
}

func readKernelVersion() string {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readHostname() string {
	data, err := os.ReadFile("/proc/sys/kernel/hostname")
	if err != nil {
		h, _ := os.Hostname()
		return h
	}
	return strings.TrimSpace(string(data))
}

func readArch() string {
	data, err := os.ReadFile("/proc/sys/kernel/arch")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func firstNonLoopbackIP() string {
	data, err := os.ReadFile("/proc/net/fib_trie")
	if err == nil {
		ip := parseFibTrieForIP(data)
		if ip != "" {
			return ip
		}
	}
	return ""
}

func parseFibTrieForIP(data []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, "/32 host LOCAL") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		ip := fields[0]
		if ip == "127.0.0.1" || ip == "0.0.0.0" {
			continue
		}
		if strings.Contains(ip, ".") {
			return ip
		}
	}
	return ""
}
