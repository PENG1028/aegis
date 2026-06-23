package handlers

import (
	"aegis/internal/manageddomain"
	"net/http"
)

func (h *Handlers) ListManagedDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := h.ManagedDomain.ListManagedDomains(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(domains))
	for i, md := range domains {
		result[i] = managedDomainToMap(md)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) CreateManagedDomain(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Domain     string `json:"domain"`
		ServiceID  string `json:"service_id"`
		OwnerRef   string `json:"owner_ref"`
		TargetType string `json:"target_type"`
		TargetRef  string `json:"target_ref"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	md, err := h.ManagedDomain.CreateManagedDomain(r.Context(), manageddomain.CreateManagedDomainInput{
		Domain: input.Domain, ServiceID: input.ServiceID,
		OwnerRef: input.OwnerRef, TargetType: input.TargetType, TargetRef: input.TargetRef,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, managedDomainToMap(*md))
}

func (h *Handlers) GetManagedDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	md, err := h.ManagedDomain.GetManagedDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, managedDomainToMap(*md))
}

func (h *Handlers) VerifyManagedDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	md, result, err := h.ManagedDomain.VerifyDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":     md.ID,
		"domain": md.Domain,
		"status": md.Status,
		"checks": result,
	})
}

func (h *Handlers) EnableManagedDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true"
	md, err := h.ManagedDomain.EnableDomain(r.Context(), id, force)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, managedDomainToMap(*md))
}

func (h *Handlers) DisableManagedDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	md, err := h.ManagedDomain.DisableDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, managedDomainToMap(*md))
}

func (h *Handlers) DeleteManagedDomain(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func managedDomainToMap(md manageddomain.ManagedDomain) map[string]interface{} {
	return map[string]interface{}{
		"id":                 md.ID,
		"domain":             md.Domain,
		"service_id":         md.ServiceID,
		"owner_ref":          md.OwnerRef,
		"target_type":        md.TargetType,
		"target_ref":         md.TargetRef,
		"verification_type":  md.VerificationType,
		"verification_name":  md.VerificationName,
		"verification_value": md.VerificationValue,
		"status":             md.Status,
		"tls_status":         md.TLSStatus,
		"last_check_message": md.LastCheckMessage,
		"created_at":         md.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":         md.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
