package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
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

	// If domain changed, regenerate Caddyfile and reload Caddy
	if domainChanged {
		caddyfilePath := h.Config.Proxy.CaddyfilePath
		if caddyfilePath == "" {
			caddyfilePath = filepath.Join(h.Config.Runtime.ConfigDir, "Caddyfile")
		}

		caddyContent := h.Config.PanelCaddyfile()
		if err := os.WriteFile(caddyfilePath, []byte(caddyContent), 0640); err != nil {
			result["caddyfile_error"] = err.Error()
		} else {
				os.Chmod(caddyfilePath, 0640)
				if grp, err := user.LookupGroup("caddy"); err == nil {
					if gid, err := strconv.Atoi(grp.Gid); err == nil {
						os.Chown(caddyfilePath, 0, gid)
					}
				}
			result["caddyfile_regenerated"] = true
			result["caddyfile_path"] = caddyfilePath

			// Reload Caddy to pick up the new config
			if out, err := exec.Command("systemctl", "reload", "caddy").CombinedOutput(); err != nil {
				result["caddy_reload_warning"] = fmt.Sprintf("Caddyfile written but reload failed: %v — %s", err, string(out))
			} else {
				result["caddy_reloaded"] = true
			}
		}

		// Tell the user the new access URL
		if h.Config.ManagedDomain.GatewayDomain != "" {
			result["panel_url"] = "https://" + h.Config.ManagedDomain.GatewayDomain
			result["tls"] = "automatic (Let's Encrypt via Caddy)"
		} else {
			result["panel_url"] = "http://<server-ip>"
			result["tls"] = "disabled (no domain configured)"
		}
	}

	writeJSON(w, http.StatusOK, result)
}
