package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"aegis/internal/core"
	"aegis/internal/endpoint"
	"aegis/internal/project"
	"aegis/internal/provider"
	"aegis/internal/route"
	"aegis/internal/service"
)

type QuickPublishRequest struct {
	Domain     string `json:"domain"`
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
}

type QuickPublishResponse struct {
	Status    string `json:"status"`
	Domain    string `json:"domain"`
	ServiceID string `json:"service_id,omitempty"`
	RouteID   string `json:"route_id,omitempty"`
	Message   string `json:"message"`
}

// AdminQuickPublish handles POST /api/admin/v1/quick-publish
// 1-click domain publishing: create service → endpoint → route → apply
func (h *Handlers) AdminQuickPublish(w http.ResponseWriter, r *http.Request) {
	var req QuickPublishRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	if req.TargetHost == "" {
		writeError(w, http.StatusBadRequest, "target_host is required")
		return
	}
	if req.TargetPort <= 0 || req.TargetPort > 65535 {
		writeError(w, http.StatusBadRequest, "target_port must be 1-65535")
		return
	}

	ctx := r.Context()

	// 1. Get or create scratch project
	projects, err := h.Project.ListProjects(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	projID := ""
	for _, p := range projects {
		if p.Name == "__quick_publish" {
			projID = p.ID
			break
		}
	}
	if projID == "" {
		p, err := h.Project.CreateProject(ctx, project.CreateProjectInput{
			Name:        "__quick_publish",
			Description: "Auto-created by quick publish",
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create project: "+err.Error())
			return
		}
		projID = p.ID
	}

	// 2. Create service
	now := time.Now()
	svcName := "qp-" + req.Domain
	svc := &service.Service{
		ID:        core.NewID("svc"),
		ProjectID: projID,
		Name:      svcName,
		Kind:      "http",
		Env:       "prod",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.Service.CreateServiceDirect(svc); err != nil {
		writeError(w, http.StatusInternalServerError, "create service: "+err.Error())
		return
	}

	// 3. Create endpoint
	addr := req.TargetHost + ":" + strconv.Itoa(req.TargetPort)
	_, err = h.EndpointSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
		ServiceID: svc.ID,
		Type:      "private",
		Address:   addr,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create endpoint: "+err.Error())
		return
	}

	// 4. Create route
	compDef := provider.LookupComp(provider.CompHTTPSRoute)
	if compDef == nil {
		writeError(w, http.StatusInternalServerError, "composition HTTPS route not found")
		return
	}
	rt := &route.Route{
		ID:          core.NewID("rt"),
		Domain:      req.Domain,
		ServiceID:   svc.ID,
		Composition: string(compDef.Key),
		TLSEnabled:  compDef.TLSMode != "none",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.Route.CreateRouteDirect(rt); err != nil {
		writeError(w, http.StatusInternalServerError, "create route: "+err.Error())
		return
	}

	// 5. Apply
	if _, err := h.Apply.Apply(ctx); err != nil {
		writeJSON(w, http.StatusOK, QuickPublishResponse{
			Status:    "failed",
			Domain:    req.Domain,
			ServiceID: svc.ID,
			RouteID:   rt.ID,
			Message:   fmt.Sprintf("domain bound but apply failed: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, QuickPublishResponse{
		Status:    "success",
		Domain:    req.Domain,
		ServiceID: svc.ID,
		RouteID:   rt.ID,
		Message:   fmt.Sprintf("published %s → %s", req.Domain, addr),
	})
}
