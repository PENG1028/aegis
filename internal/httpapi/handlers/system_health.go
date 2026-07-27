package handlers

import (
	"net/http"
	"os"
	"runtime"
)

// PortScanResponse holds the result of scanning OS ports for conflicts.
type PortScanResponse struct {
	Conflicts []PortConflictEntry `json:"conflicts"`
	Total     int                 `json:"total"`
}

type PortConflictEntry struct {
	Port    int    `json:"port"`
	BindIP  string `json:"bind_ip"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// PortScan runs a system-level port scan and reports ports not managed by Aegis.
func (h *Handlers) PortScan(w http.ResponseWriter, r *http.Request) {
	resp := PortScanResponse{
		Conflicts: []PortConflictEntry{},
	}

	if h.ListenerSvc == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	raw, err := h.ListenerSvc.CheckOSPorts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "port scan: "+err.Error())
		return
	}

	for _, rc := range raw {
		resp.Conflicts = append(resp.Conflicts, PortConflictEntry{
			Port:    rc.Port,
			BindIP:  rc.BindIP,
			Status:  rc.Status,
			Message: rc.Message,
		})
	}
	resp.Total = len(resp.Conflicts)

	writeJSON(w, http.StatusOK, resp)
}

// SystemHealthResponse holds key system-level health metrics.
type SystemHealthResponse struct {
	SQLiteOK        bool   `json:"sqlite_ok"`
	SQLiteSizeBytes int64  `json:"sqlite_size_bytes"`
	DiskFreeBytes   int64  `json:"disk_free_bytes"`
	DiskTotalBytes  int64  `json:"disk_total_bytes"`
	MemoryUsedMB    int64  `json:"memory_used_mb"`
	MemoryTotalMB   int64  `json:"memory_total_mb"`
	GoVersion       string `json:"go_version"`
	Goroutines      int    `json:"goroutines"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
}

// SystemHealth runs key system health checks — SQLite integrity, disk space,
// memory usage, and Go runtime stats — and returns a single summary.
func (h *Handlers) SystemHealth(w http.ResponseWriter, r *http.Request) {
	resp := SystemHealthResponse{
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
	}

	// SQLite integrity check
	if h.DB != nil {
		var ok string
		if err := h.DB.QueryRow("PRAGMA integrity_check").Scan(&ok); err == nil && ok == "ok" {
			resp.SQLiteOK = true
		}
	}

	// SQLite file size
	if h.Config != nil && h.Config.Store.SQLitePath != "" {
		if fi, err := os.Stat(h.Config.Store.SQLitePath); err == nil {
			resp.SQLiteSizeBytes = fi.Size()
		}
	}

	// Platform-specific: disk, memory, uptime
	h.fillSystemHealthPlatform(&resp)

	writeJSON(w, http.StatusOK, resp)
}
