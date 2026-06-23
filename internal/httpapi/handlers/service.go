package handlers

import (
	"aegis/internal/service"
	"net/http"
)

func (h *Handlers) ListServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.Service.ListServices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(services))
	for i, s := range services {
		result[i] = serviceToMap(s)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) CreateService(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProjectID string `json:"project_id"`
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		Env       string `json:"env"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s, err := h.Service.CreateService(r.Context(), service.CreateServiceInput{
		ProjectID: input.ProjectID, Name: input.Name, Kind: input.Kind, Env: input.Env,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, serviceToMap(*s))
}

func (h *Handlers) GetService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.Service.GetService(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, serviceToMap(*s))
}

func (h *Handlers) UpdateService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		Kind *string `json:"kind"`
		Env  *string `json:"env"`
		Note *string `json:"note"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s, err := h.Service.UpdateService(r.Context(), id, service.UpdateServiceInput{
		Kind: input.Kind, Env: input.Env, Note: input.Note,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, serviceToMap(*s))
}

func (h *Handlers) EnableService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Service.EnableService(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (h *Handlers) DisableService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Service.DisableService(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func serviceToMap(s service.Service) map[string]interface{} {
	return map[string]interface{}{
		"id":         s.ID,
		"project_id": s.ProjectID,
		"name":       s.Name,
		"kind":       s.Kind,
		"env":        s.Env,
		"status":     s.Status,
		"note":       s.Note,
		"created_at": s.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at": s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
