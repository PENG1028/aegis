//go:build linux

package handlers

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// fillSystemHealthPlatform fills in the Linux-specific fields of SystemHealthResponse.
func (h *Handlers) fillSystemHealthPlatform(resp *SystemHealthResponse) {
	// Disk space on the data directory
	dataDir := "/var/lib/aegis"
	if h.Config != nil && h.Config.Runtime.DataDir != "" {
		dataDir = h.Config.Runtime.DataDir
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(dataDir, &stat); err == nil {
		resp.DiskFreeBytes = int64(stat.Bavail) * int64(stat.Bsize)
		resp.DiskTotalBytes = int64(stat.Blocks) * int64(stat.Bsize)
	}

	// Memory (via /proc/meminfo)
	memTotal, memAvail := readMemInfo()
	resp.MemoryTotalMB = memTotal / 1024
	if memTotal > 0 && memAvail >= 0 {
		resp.MemoryUsedMB = (memTotal - memAvail) / 1024
	}

	// Uptime (via /proc/uptime)
	resp.UptimeSeconds = readUptime()
}

func readMemInfo() (totalKB int64, availKB int64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v := parseKbLine(line, "MemTotal:"); v > 0 && totalKB == 0 {
			totalKB = v
		}
		if v := parseKbLine(line, "MemAvailable:"); v > 0 && availKB == 0 {
			availKB = v
		}
	}
	return
}

func parseKbLine(line, prefix string) int64 {
	if !strings.HasPrefix(line, prefix) {
		return 0
	}
	rest := strings.TrimSpace(line[len(prefix):])
	rest = strings.TrimSuffix(rest, " kB")
	rest = strings.TrimSpace(rest)
	v, _ := strconv.ParseInt(rest, 10, 64)
	return v
}

func readUptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	if sec, err := strconv.ParseFloat(fields[0], 64); err == nil {
		return int64(sec)
	}
	return 0
}
