package handlers

import (
	"encoding/json"
	"net/http"

	providerpkg "aegis/internal/hostdep/provider"
)

// capabilityStatus is the common response shape for resource capability queries.
type capabilityStatus struct {
	Operations map[string]capOp `json:"operations"`
}

type capOp struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// AdminRouteCapabilityStatus handles GET /api/admin/v1/routes/{id}/capability-status
func (h *Handlers) AdminRouteCapabilityStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rt, err := h.Route.GetRoute(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	provider := rt.SourceProvider
	if provider == "" {
		provider = "unassigned"
	}
	migrationAvailable := false
	if h.ProvReg != nil {
		for _, mode := range providerpkg.AllRuntimeModes() {
			target, _ := routeExecutionForMode(*rt, mode, h.ProvReg)
			if target != "" && target != currentRouteExecutor(*rt) {
				migrationAvailable = true
				break
			}
		}
	}
	semantics := make(map[string]providerpkg.CapabilitySemantics)
	for _, capability := range rt.CapabilityKeys() {
		semantics[capability] = providerpkg.SemanticsOf(providerpkg.Capability(capability))
	}

	resp := map[string]interface{}{
		"route_id":             rt.ID,
		"provider":             provider,
		"capabilities":         parseCapJSON(rt.SourceCapabilities),
		"capability_semantics": semantics,
		"operations": map[string]capOp{
			"modify": {Available: true},
			"delete": {Available: true},
			"migrate_execution": {
				Available: migrationAvailable,
				Reason:    "",
			},
		},
		"cert_ref": map[string]interface{}{
			"id": nil,
		},
	}

	if rt.CertID != nil && *rt.CertID != "" {
		cert, _ := h.CertStore.Get(*rt.CertID)
		mapped := map[string]interface{}{"id": *rt.CertID}
		if cert != nil {
			mapped["source"] = cert.Source
			mapped["managed_by"] = certManagedBy(cert.Source)
		}
		resp["cert_ref"] = mapped
	}

	writeJSON(w, http.StatusOK, resp)
}

// AdminCertCapabilityStatus handles GET /api/admin/v1/certificates/{id}/capability-status
func (h *Handlers) AdminCertCapabilityStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cert, err := h.CertStore.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if cert == nil {
		writeError(w, http.StatusNotFound, "certificate not found")
		return
	}

	managedBy := certManagedBy(cert.Source)

	// Count routes that reference this cert
	refs, _ := h.Route.FindRoutesByCertID(r.Context(), id)
	refCount := len(refs)
	deleteAvailable := certCanDelete(cert.Source) && refCount == 0

	var refIDs []string
	for _, ref := range refs {
		refIDs = append(refIDs, ref.ID)
	}

	resp := map[string]interface{}{
		"cert_id":       cert.ID,
		"source":        cert.Source,
		"managed_by":    managedBy,
		"ref_count":     refCount,
		"ref_route_ids": refIDs,
		"operations": map[string]capOp{
			"delete": {
				Available: deleteAvailable,
				Reason:    certDeleteReason(cert.Source, refCount),
			},
			"renew": {
				Available: cert.Source == "local_acme",
				Reason:    certRenewReason(cert.Source),
			},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

func certRenewReason(source string) string {
	if source == "gateway_auto" {
		return "renewal is managed internally by the automatic TLS executor"
	}
	if source != "local_acme" {
		return "this certificate must be replaced manually"
	}
	return ""
}

func certManagedBy(source string) string {
	switch source {
	case "gateway_auto":
		return "executor"
	case "local_acme":
		return "aegis"
	case "manual_upload", "external":
		return "user"
	default:
		return "unknown"
	}
}

func certCanDelete(source string) bool {
	return source != "gateway_auto"
}

func certDeleteReason(source string, refCount int) string {
	if source == "gateway_auto" {
		return "自动 TLS 状态随入口策略管理，不能作为独立证书资产删除"
	}
	if refCount > 0 {
		return "certificate is referenced by routes — unbind first"
	}
	return ""
}

func parseCapJSON(raw string) []string {
	if raw == "" {
		return nil
	}
	var caps []string
	json.Unmarshal([]byte(raw), &caps)
	return caps
}
