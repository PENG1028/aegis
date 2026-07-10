package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aegis/internal/endpoint"
	"aegis/internal/service"
)

func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	// Sanitize server config — never expose the admin token
	serverSafe := h.Config.Server
	if len(serverSafe.AdminToken) > 8 {
		serverSafe.AdminToken = serverSafe.AdminToken[:4] + "..." + serverSafe.AdminToken[len(serverSafe.AdminToken)-4:]
	} else {
		serverSafe.AdminToken = "***REDACTED***"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"proxy":          h.Config.Proxy,
		"store":          h.Config.Store,
		"server":         serverSafe,
		"managed_domain": h.Config.ManagedDomain,
		"runtime":        h.Config.Runtime,
	})
}

// UpdateSettings handles PATCH /api/settings.
//
// Allowed fields:
//   - managed_domain.gateway_domain — sets the panel domain, triggers Caddyfile regen + reload
//   - proxy.email — Let's Encrypt notification email
//
// Security: admin_token, sqlite_path, and other critical fields are NOT updatable
// via this endpoint. They must be changed in config.yaml directly.
func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	changed := false
	domainChanged := false

	// ─── managed_domain.gateway_domain ───
	if mdRaw, ok := req["managed_domain"]; ok {
		md, ok := mdRaw.(map[string]interface{})
		if !ok {
			writeError(w, http.StatusBadRequest, "managed_domain must be an object")
			return
		}
		if domain, ok := md["gateway_domain"]; ok {
			domainStr, ok := domain.(string)
			if !ok {
				writeError(w, http.StatusBadRequest, "gateway_domain must be a string")
				return
			}
			// Basic validation: empty or valid domain
			domainStr = strings.TrimSpace(domainStr)
			if domainStr != "" && !strings.Contains(domainStr, ".") {
				writeError(w, http.StatusBadRequest, "gateway_domain must be a valid domain (e.g. panel.example.com)")
				return
			}
			if domainStr != h.Config.ManagedDomain.GatewayDomain {
				h.Config.ManagedDomain.GatewayDomain = domainStr
				changed = true
				domainChanged = true
			}
		}
	}

	// ─── proxy.email / tls cert/key (content or file path) ───
	if proxyRaw, ok := req["proxy"]; ok {
		proxy, ok := proxyRaw.(map[string]interface{})
		if !ok {
			writeError(w, http.StatusBadRequest, "proxy must be an object")
			return
		}
		if email, ok := proxy["email"]; ok {
			emailStr, ok := email.(string)
			if !ok {
				writeError(w, http.StatusBadRequest, "email must be a string")
				return
			}
			emailStr = strings.TrimSpace(emailStr)
			if emailStr != h.Config.Proxy.Email {
				h.Config.Proxy.Email = emailStr
				changed = true
			}
		}

		// TLS cert: accept content (paste) or file path.
		// Content takes priority — it gets saved to /etc/aegis/certs/ automatically.
		if certContent, ok := proxy["tls_cert_content"]; ok {
			s, _ := certContent.(string)
			s = strings.TrimSpace(s)
			if s != "" {
				certsDir := filepath.Join(h.Config.Runtime.ConfigDir, "certs")
				os.MkdirAll(certsDir, 0700)
				certPath := filepath.Join(certsDir, "panel.crt")
				if err := os.WriteFile(certPath, []byte(s+"\n"), 0600); err != nil {
					writeError(w, http.StatusInternalServerError, "save cert: "+err.Error())
					return
				}
				h.Config.Proxy.TlsCertFile = certPath
				changed = true
				domainChanged = true
			}
		} else if certFile, ok := proxy["tls_cert_file"]; ok {
			s, _ := certFile.(string)
			s = strings.TrimSpace(s)
			if s != h.Config.Proxy.TlsCertFile {
				h.Config.Proxy.TlsCertFile = s
				changed = true
				domainChanged = true
			}
		}

		if keyContent, ok := proxy["tls_key_content"]; ok {
			s, _ := keyContent.(string)
			s = strings.TrimSpace(s)
			if s != "" {
				certsDir := filepath.Join(h.Config.Runtime.ConfigDir, "certs")
				os.MkdirAll(certsDir, 0700)
				keyPath := filepath.Join(certsDir, "panel.key")
				if err := os.WriteFile(keyPath, []byte(s+"\n"), 0600); err != nil {
					writeError(w, http.StatusInternalServerError, "save key: "+err.Error())
					return
				}
				h.Config.Proxy.TlsKeyFile = keyPath
				changed = true
				domainChanged = true
			}
		} else if keyFile, ok := proxy["tls_key_file"]; ok {
			s, _ := keyFile.(string)
			s = strings.TrimSpace(s)
			if s != h.Config.Proxy.TlsKeyFile {
				h.Config.Proxy.TlsKeyFile = s
				changed = true
				domainChanged = true
			}
		}
	}

	if !changed {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "unchanged",
			"message": "no changes to apply",
		})
		return
	}

	// Save config to disk
	configPath := filepath.Join(h.Config.Runtime.ConfigDir, "config.yaml")
	if err := h.Config.Save(configPath); err != nil {
		writeError(w, http.StatusInternalServerError, "save config: "+err.Error())
		return
	}

	result := map[string]interface{}{
		"status":             "updated",
		"message":            "settings saved",
		"gateway_domain":     h.Config.ManagedDomain.GatewayDomain,
		"config_path":        configPath,
	}

	// If domain changed, ensure panel service + endpoint exist,
	// then create a system route via the Apply pipeline.
	// Panel domain now goes through: service+endpoint → route → Apply → planner → render → reload
	if domainChanged {
		if domain := h.Config.ManagedDomain.GatewayDomain; domain != "" {
			// Ensure the __panel service exists for the planner to resolve
			if err := h.ensurePanelService(r.Context()); err != nil {
				result["panel_service_warning"] = err.Error()
			}
			// Ensure the panel endpoint (127.0.0.1:7380) exists
			if err := h.ensurePanelEndpoint(r.Context()); err != nil {
				result["panel_endpoint_warning"] = err.Error()
			}
			h.Route.UpsertSystemRoute(r.Context(), domain)
		}

		// Run Apply pipeline — handles rendering, validation, Caddy reload
		if _, err := h.Apply.Apply(r.Context()); err != nil {
			result["apply_warning"] = fmt.Sprintf("route created but apply failed: %v", err)
		} else {
			result["apply_success"] = true
		}

		// Tell the user the new access URL
		if domain := h.Config.ManagedDomain.GatewayDomain; domain != "" {
			result["panel_url"] = "https://" + domain
			result["tls"] = "automatic (Let's Encrypt via Caddy)"
		} else {
			result["panel_url"] = "http://<server-ip>"
			result["tls"] = "disabled (no domain configured)"
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// ensurePanelService creates the __panel service if it doesn't exist.
// The planner needs a real service to resolve the panel route.
func (h *Handlers) ensurePanelService(ctx context.Context) error {
	svc, _ := h.Service.GetService(ctx, "__panel")
	if svc != nil {
		return nil
	}
	now := time.Now()
	return h.Service.CreateServiceDirect(&service.Service{
		ID:        "__panel",
		ProjectID: "__system",
		Name:      "Aegis Panel",
		Kind:      "http",
		Env:       "prod",
		Status:    "active",
		SpaceID:   "default",
		OwnerType: "system",
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// ensurePanelEndpoint creates the panel endpoint (127.0.0.1:7380) if it doesn't exist.
// The planner needs a real endpoint to determine the upstream address.
func (h *Handlers) ensurePanelEndpoint(ctx context.Context) error {
	eps, _ := h.EndpointRepo.FindByServiceID("__panel")
	if len(eps) > 0 {
		return nil
	}
	_, err := h.EndpointSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
		ServiceID: "__panel",
		Type:      "local",
		Address:   "127.0.0.1:7380",
	})
	return err
}
