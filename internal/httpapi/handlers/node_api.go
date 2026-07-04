package handlers

import (
	"net/http"
	"strings"

	"aegis/internal/gateway"
	"aegis/internal/node"
	"aegis/internal/nodeauth"
)

// NodeJoin handles POST /api/node/v1/join
// This endpoint is intentionally public — it uses the join token in the request body for auth.
func (h *Handlers) NodeJoin(w http.ResponseWriter, r *http.Request) {
	var req nodeauth.JoinRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.JoinToken == "" {
		writeError(w, http.StatusBadRequest, "join_token is required")
		return
	}
	if req.NodeName == "" {
		req.NodeName = req.Hostname
	}
	if req.NodeName == "" {
		writeError(w, http.StatusBadRequest, "node_name or hostname is required")
		return
	}

	sourceIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		sourceIP = forwarded
	}

	resp, err := h.NodeAuthSvc.RegisterNode(req, sourceIP)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"node_id":                resp.NodeID,
		"node_token":             resp.NodeToken,
		"node_token_redacted":    resp.NodeTokenRedacted,
		"status":                 resp.Status,
		"heartbeat_after_seconds": resp.HeartbeatAfter,
		"warning":                "store this token securely — it will not be shown again",
	})
}

// authenticateNodeRequest extracts and validates the node credential from a request.
func (h *Handlers) authenticateNodeRequest(w http.ResponseWriter, r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		writeError(w, http.StatusUnauthorized, "missing Authorization header")
		return ""
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		writeError(w, http.StatusUnauthorized, "invalid Authorization format, expected 'Bearer <token>'")
		return ""
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		writeError(w, http.StatusUnauthorized, "empty node credential token")
		return ""
	}

	nodeID, err := h.NodeAuthSvc.AuthenticateNode(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return ""
	}

	return nodeID
}

// NodeHeartbeat handles POST /api/node/v1/heartbeat
func (h *Handlers) NodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	authNodeID := h.authenticateNodeRequest(w, r)
	if authNodeID == "" {
		return
	}

	var req nodeauth.HeartbeatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.NodeID == "" {
		writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	if req.NodeID != authNodeID {
		writeError(w, http.StatusForbidden, "node credential does not match requested node_id")
		return
	}

	validStatus := map[string]bool{
		node.StatusOnline:   true,
		node.StatusOffline:  true,
		node.StatusDegraded: true,
		node.StatusUnknown:  true,
	}
	if !validStatus[req.Status] {
		req.Status = node.StatusOnline
	}

	if err := h.NodeSvc.UpdateHeartbeat(
		req.NodeID, req.Status, req.AgentVersion, req.PublicIP,
		req.PrivateIP, req.Hostname, req.LastError,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record heartbeat: "+err.Error())
		return
	}

	if len(req.Capabilities) > 0 {
		caps := make(node.NodeCapabilities)
		for _, c := range req.Capabilities {
			caps[c] = true
		}
		_ = h.NodeRepo.UpdateCapabilities(req.NodeID, caps)
	}

	// v1.8C-2A: Process gateway status from heartbeat
	if len(req.Gateways) > 0 {
		for i := range req.Gateways {
			gw := req.Gateways[i]
			if gw.GatewayID != "" {
				// Update existing gateway by ID with ownership enforcement
				update := gateway.GatewayInventory{
					Host:              gw.Host,
					Port:              gw.Port,
					Scheme:            gw.Scheme,
					BindAddr:          gw.BindAddr,
					PublicAccessible:  gw.PublicAccessible,
					PrivateAccessible: gw.PrivateAccessible,
					Enabled:           gw.Enabled,
					Status:            gw.Status,
					LastError:         gw.LastError,
				}
				if err := h.GatewayInvSvc.UpdateGatewayFromHeartbeat(req.NodeID, gw.GatewayID, update); err != nil {
					// Log but don't fail the heartbeat — gateway status is advisory
					_ = err
				}
			} else if gw.Name != "" {
				// Upsert gateway by name
				upsert := gateway.GatewayInventory{
					Name:              gw.Name,
					Type:              gw.Type,
					Provider:          gw.Provider,
					Host:              gw.Host,
					Port:              gw.Port,
					Scheme:            gw.Scheme,
					BindAddr:          gw.BindAddr,
					PublicAccessible:  gw.PublicAccessible,
					PrivateAccessible: gw.PrivateAccessible,
					Enabled:           gw.Enabled,
					Status:            gw.Status,
					LastError:         gw.LastError,
				}
				if err := h.GatewayInvSvc.UpsertGatewayFromHeartbeat(req.NodeID, upsert); err != nil {
					_ = err
				}
			}
		}
	}

	// v1.8C-2: Revision hint from desired state
	latestRev, desiredAvail, outdated, _ := h.NodeStateSvc.CompareNodeRevision(req.NodeID, req.AppliedRevision)

	// v1.8L: Check for pending binary update
	var updateAvail *nodeauth.UpdateInfo
	if ui := GetNodeUpdatePending(req.NodeID); ui != nil {
		updateAvail = &nodeauth.UpdateInfo{
			Version:  ui.Version,
			Checksum: ui.Checksum,
			Size:     ui.Size,
		}
	}

	writeJSON(w, http.StatusOK, nodeauth.HeartbeatResponse{
		NodeID:            req.NodeID,
		Status:            "accepted",
		LatestRevision:    latestRev,
		DesiredStateAvail: desiredAvail,
		NodeIsOutdated:    outdated,
		UpdateAvailable:   updateAvail,
	})
}

// NodeGatewayLinkToken handles GET /api/node/v1/gateway-link-token/{gatewayLinkID}
// Node auth required. Returns the decrypted GatewayLink secret for runtime injection.
// Master key missing or decryption failure returns 404/503 to prevent token leak.
func (h *Handlers) NodeGatewayLinkToken(w http.ResponseWriter, r *http.Request) {
	authNodeID := h.authenticateNodeRequest(w, r)
	if authNodeID == "" {
		return
	}

	gatewayLinkID := r.PathValue("gatewayLinkID")
	if gatewayLinkID == "" {
		writeError(w, http.StatusBadRequest, "gateway link ID is required")
		return
	}

	token, err := h.GatewayLinkSvc.GetDecryptedSecret(gatewayLinkID)
	if err != nil {
		// Don't leak whether the link exists or the key is missing
		writeError(w, http.StatusNotFound, "gateway link token unavailable")
		return
	}
	if token == "" {
		writeError(w, http.StatusNotFound, "gateway link token unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}
