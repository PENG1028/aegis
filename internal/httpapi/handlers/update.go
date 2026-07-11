package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

const (
	updateDir      = "/var/lib/aegis/updates"
	updateBinary   = "/var/lib/aegis/updates/aegis-latest"
	updateChecksum = "/var/lib/aegis/updates/aegis-latest.sha256"
)

// pendingUpdates tracks which nodes have a pending binary update.
// The node's heartbeat response includes update info when pending.
var (
	pendingUpdates   = make(map[string]*UpdateInfo)
	pendingUpdatesMu sync.Mutex
)

// UpdateInfo contains the binary update details sent to the node.
type UpdateInfo struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
}

// MarkNodeUpdatePending marks a node for binary update.
// The node will receive update info in its next heartbeat response.
func MarkNodeUpdatePending(nodeID, version, checksum string, size int64) {
	pendingUpdatesMu.Lock()
	pendingUpdates[nodeID] = &UpdateInfo{Version: version, Checksum: checksum, Size: size}
	pendingUpdatesMu.Unlock()
}

// GetNodeUpdatePending returns the pending update info for a node, if any.
// Returns nil if no update is pending.
func GetNodeUpdatePending(nodeID string) *UpdateInfo {
	pendingUpdatesMu.Lock()
	defer pendingUpdatesMu.Unlock()
	info, ok := pendingUpdates[nodeID]
	if !ok {
		return nil
	}
	// Once delivered to the node, clear it (node will ack via actual-state)
	return info
}

// ClearNodeUpdatePending clears the pending update for a node (called on ack).
func ClearNodeUpdatePending(nodeID string) {
	pendingUpdatesMu.Lock()
	delete(pendingUpdates, nodeID)
	pendingUpdatesMu.Unlock()
}

// UploadBinary handles POST /api/admin/v1/system/upload-binary
// Receives a new aegis binary for distribution to nodes.
func (h *Handlers) UploadBinary(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := os.MkdirAll(updateDir, 0700); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot create update dir: "+err.Error())
		return
	}

	tmpFile := updateBinary + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot create temp file: "+err.Error())
		return
	}

	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)
	written, err := io.Copy(writer, r.Body)
	if err != nil {
		f.Close()
		os.Remove(tmpFile)
		writeError(w, http.StatusBadRequest, "upload failed: "+err.Error())
		return
	}
	f.Close()

	if written < 1<<20 {
		os.Remove(tmpFile)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("binary too small (%d bytes)", written))
		return
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	if err := os.WriteFile(updateChecksum, []byte(checksum+"\n"), 0600); err != nil {
		os.Remove(tmpFile)
		writeError(w, http.StatusInternalServerError, "cannot write checksum: "+err.Error())
		return
	}

	if err := os.Rename(tmpFile, updateBinary); err != nil {
		os.Remove(tmpFile)
		writeError(w, http.StatusInternalServerError, "cannot finalize upload: "+err.Error())
		return
	}
	os.Chmod(updateBinary, 0755)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "uploaded",
		"size":     written,
		"checksum": checksum,
	})
}

// ServeBinary handles GET /api/node/v1/binary
// Nodes download the latest binary from the control plane.
// Requires node credential auth (Bearer token) — same as other node endpoints.
func (h *Handlers) ServeBinary(w http.ResponseWriter, r *http.Request) {
	if h.NodeAuthSvc == nil {
		writeError(w, http.StatusInternalServerError, "node auth service not configured")
		return
	}
// authenticateNodeRequest removed
	if _, err := os.Stat(updateBinary); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "no update binary available")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=aegis")
	http.ServeFile(w, r, updateBinary)
}

// BinaryInfo handles GET /api/admin/v1/system/binary-info
func (h *Handlers) BinaryInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{"available": false}
	if stat, err := os.Stat(updateBinary); err == nil {
		info["available"] = true
		info["size"] = stat.Size()
		info["mod_time"] = stat.ModTime().UTC().Format("2006-01-02T15:04:05Z")
	}
	if cs, err := os.ReadFile(updateChecksum); err == nil {
		info["checksum"] = strings.TrimSpace(string(cs))
	}
	writeJSON(w, http.StatusOK, info)
}

// TriggerNodeUpdate handles POST /api/admin/v1/nodes/{id}/update
// Marks the node for binary update. Node picks it up on next heartbeat.
func (h *Handlers) TriggerNodeUpdate(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	stat, err := os.Stat(updateBinary)
	if os.IsNotExist(err) {
		writeError(w, http.StatusBadRequest, "no update binary uploaded. Upload via POST /api/admin/v1/system/upload-binary first.")
		return
	}

	cs, _ := os.ReadFile(updateChecksum)
	checksum := strings.TrimSpace(string(cs))

	MarkNodeUpdatePending(nodeID, h.Version, checksum, stat.Size())

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "triggered",
		"node_id":  nodeID,
		"version":  h.Version,
		"checksum": checksum,
		"message":  fmt.Sprintf("Update triggered for node %s. Node will self-update on next heartbeat.", nodeID),
	})
}

// PendingUpdatesList handles GET /api/admin/v1/system/pending-updates
func (h *Handlers) PendingUpdatesList(w http.ResponseWriter, r *http.Request) {
	pendingUpdatesMu.Lock()
	nodes := make([]string, 0, len(pendingUpdates))
	for id := range pendingUpdates {
		nodes = append(nodes, id)
	}
	pendingUpdatesMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pending_nodes": nodes,
		"count":         len(nodes),
	})
}
