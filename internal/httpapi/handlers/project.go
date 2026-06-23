package handlers

import (
	"aegis/internal/project"
	"net/http"
)

func (h *Handlers) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.Project.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(projects))
	for i, p := range projects {
		result[i] = projectToMap(p)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) CreateProject(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := h.Project.CreateProject(r.Context(), project.CreateProjectInput{
		Name: input.Name, Description: input.Description,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, projectToMap(*p))
}

func (h *Handlers) GetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.Project.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projectToMap(*p))
}

func (h *Handlers) UpdateProject(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (h *Handlers) ArchiveProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Project.ArchiveProject(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

func projectToMap(p project.Project) map[string]interface{} {
	return map[string]interface{}{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"status":      p.Status,
		"created_at":  p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":  p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
