package handlers

import (
	"net/http"
	"path/filepath"
	"time"

	"aegis/internal/endpoint"
	"aegis/internal/importcfg"
	"aegis/internal/id"
	"aegis/internal/route"
	"aegis/internal/service"
)

// AdminImportCaddyPreview handles GET /api/admin/v1/import/caddy/preview
// Scans the Caddyfile and returns parsed routes without writing.
func (h *Handlers) AdminImportCaddyPreview(w http.ResponseWriter, r *http.Request) {
	paths := importcfg.FindCaddyfiles()
	if len(paths) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"routes": []importcfg.ImportedRoute{},
			"count":  0,
			"message": "no Caddyfile found in standard locations",
		})
		return
	}

	var allRoutes []importcfg.ImportedRoute
	var allErrors []string

	for _, caddyPath := range paths {
		result, err := importcfg.ScanCaddyfile(caddyPath)
		if err != nil {
			allErrors = append(allErrors, caddyPath+": "+err.Error())
			continue
		}
		// Mark the source file
		for i := range result.Routes {
			result.Routes[i].SourceFile = filepath.Base(caddyPath)
		}
		allRoutes = append(allRoutes, result.Routes...)
		allErrors = append(allErrors, result.Errors...)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"routes": allRoutes,
		"count":  len(allRoutes),
		"errors": allErrors,
	})
}

// AdminImportCaddyConfirm handles POST /api/admin/v1/import/caddy/confirm
// Writes parsed Caddyfile routes into the Aegis database.
// Accepts a list of routes to import (the user can select which ones).
func (h *Handlers) AdminImportCaddyConfirm(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Routes []importcfg.ImportedRoute `json:"routes"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(input.Routes) == 0 {
		writeError(w, http.StatusBadRequest, "no routes to import")
		return
	}

	var imported []map[string]interface{}
	var importErrors []string

	for _, ir := range input.Routes {
		// Map domain to a service name
		svcName := sanitizeName(ir.Domain) + "-imported"

		// Use the first project available
		projects, err := h.Project.ListProjects(r.Context())
		if err != nil || len(projects) == 0 {
			importErrors = append(importErrors, ir.Domain+": no project available")
			continue
		}
		projectID := projects[0].ID

		// Create service
		now := time.Now()
		svc := &service.Service{
			ID:        id.New("svc"),
			ProjectID: projectID,
			Name:      svcName,
			Kind:      "http",
			Env:       "prod",
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := h.Service.CreateServiceDirect(svc); err != nil {
			importErrors = append(importErrors, ir.Domain+": create service: "+err.Error())
			continue
		}

		// Create endpoint
		ep := &endpoint.Endpoint{
			ID:        id.New("ep"),
			ServiceID: svc.ID,
			Type:      "local",
			Address:   ir.UpstreamURL,
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := h.EndpointRepo.Create(ep); err != nil {
			importErrors = append(importErrors, ir.Domain+": create endpoint: "+err.Error())
			continue
		}

		// Create route
		rt := &route.Route{
			ID:         id.New("rt"),
			Domain:     ir.Domain,
			PathPrefix: ir.PathPrefix,
			ServiceID:  svc.ID,
			TLSEnabled: ir.TLSEnabled,
			StripPrefix: ir.StripPrefix,
			Status:     "active",
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := h.Route.CreateRouteDirect(rt); err != nil {
			importErrors = append(importErrors, ir.Domain+": create route: "+err.Error())
			continue
		}

		// Mark pending
		if h.PendingState != nil {
			h.PendingState.MarkPending("caddy import: " + ir.Domain)
		}

		imported = append(imported, map[string]interface{}{
			"domain":     ir.Domain,
			"service_id": svc.ID,
			"route_id":   rt.ID,
			"status":     "imported",
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported": imported,
		"count":    len(imported),
		"errors":   importErrors,
		"pending_apply": h.PendingState != nil,
	})
}

// sanitizeName converts a domain to a valid service name.
func sanitizeName(domain string) string {
	cleaned := make([]byte, 0, len(domain))
	for i := 0; i < len(domain); i++ {
		c := domain[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			cleaned = append(cleaned, c)
		} else if c >= 'A' && c <= 'Z' {
			cleaned = append(cleaned, c+32) // to lowercase
		} else if c == '.' {
			cleaned = append(cleaned, '-')
		}
	}
	return string(cleaned)
}
