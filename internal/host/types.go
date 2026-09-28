package host

import "time"

// Snapshot is a point-in-time view of this node for the API.
type Snapshot struct {
	Timestamp time.Time `json:"timestamp"`

	Hostname string `json:"hostname"`
	Kernel   string `json:"kernel"`
	Arch     string `json:"arch"`
	Uptime   int64  `json:"uptimeSeconds"`

	CPU CPUMetrics `json:"cpu"`
	RAM RAMMetrics `json:"ram"`
	Disk DiskMetrics `json:"disk"`

	TemperatureC *float64 `json:"temperatureC,omitempty"`

	Network []NetInterface `json:"network"`

	Device DeviceRow `json:"device"`
}

type CPUMetrics struct {
	UsagePercent float64   `json:"usagePercent"`
	Load1        float64   `json:"load1"`
	Load5        float64   `json:"load5"`
	Load15       float64   `json:"load15"`
}

type RAMMetrics struct {
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	UsagePercent   float64 `json:"usagePercent"`
}

type DiskMetrics struct {
	Path           string  `json:"path"`
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	UsagePercent   float64 `json:"usagePercent"`
}

type NetInterface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	RxBytes   uint64   `json:"rxBytes"`
	TxBytes   uint64   `json:"txBytes"`
	RxBps     float64  `json:"rxBps"`
	TxBps     float64  `json:"txBps"`
}

type DeviceRow struct {
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	IP     string  `json:"ip"`
	Status string  `json:"status"`
	Load   float64 `json:"loadPercent"`
}
