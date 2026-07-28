package handlers

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"aegis/internal/certstore"
	"aegis/internal/hostdep/tool"
	"aegis/internal/route"
	"aegis/internal/tlslifecycle"
)

// ─── Certificate handlers ───

// AdminListCertificates returns certificate assets plus read-only provider
// observations. Listing never copies provider keys or mutates route bindings.
func (h *Handlers) AdminListCertificates(w http.ResponseWriter, r *http.Request) {
	var assets []map[string]interface{}
	var automaticTLS []map[string]interface{}
	var routeList []route.Route
	if h.Route != nil {
		var err error
		routeList, err = h.Route.ListRoutes(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list certificate references: "+err.Error())
			return
		}
	}
	if h.CertStore != nil {
		certs, err := h.CertStore.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list certificates: "+err.Error())
			return
		}
		for _, c := range certs {
			if c.Source == certstore.SourceGatewayAuto {
				continue // legacy imported copies are superseded by provider observations
			}
			var refs []route.Route
			if h.Route != nil {
				refs, err = h.Route.FindRoutesByCertID(r.Context(), c.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "list certificate references: "+err.Error())
					return
				}
			}
			assets = append(assets, map[string]interface{}{
				"id":          c.ID,
				"domains":     c.Domains,
				"issuer":      c.Issuer,
				"not_before":  c.NotBefore,
				"not_after":   c.NotAfter,
				"cert_path":   c.CertPath,
				"key_path":    c.KeyPath,
				"source":      c.Source,
				"note":        c.Note,
				"managed":     true,
				"managed_by":  certManagedBy(c.Source),
				"record_type": "asset",
				"ref_count":   len(refs),
				"auto_renew":  c.Source == certstore.SourceLocalACME,
				"created_at":  c.CreatedAt,
			})
		}
	}

	var observationWarnings []string
	for _, observer := range h.TLSObservers {
		discovered, err := observer.Observe(r.Context())
		if err != nil {
			observationWarnings = append(observationWarnings, observer.ExecutorID()+": "+err.Error())
		}
		for _, c := range discovered {
			executorID := observer.ExecutorID()
			sum := sha256.Sum256([]byte(executorID + "|" + c.Domains + "|" + c.ACMEPath))
			refCount := 0
			observation := &certstore.Certificate{Domains: c.Domains}
			for _, rt := range routeList {
				if rt.TLSBindingMode == route.TLSBindingProviderAuto &&
					(rt.TLSProvider == "" || rt.TLSProvider == executorID) &&
					certstore.CoversDomain(observation, rt.Domain) {
					refCount++
				}
			}
			automaticTLS = append(automaticTLS, map[string]interface{}{
				"id":          fmt.Sprintf("managed_%x", sum[:8]),
				"domains":     c.Domains,
				"issuer":      c.Issuer,
				"not_before":  c.NotBefore,
				"not_after":   c.NotAfter,
				"source":      certstore.SourceGatewayAuto,
				"note":        "Observed in automatic TLS executor storage",
				"managed":     true,
				"managed_by":  executorID,
				"record_type": "provider_observation",
				"ref_count":   refCount,
				"auto_renew":  true,
			})
		}
	}

	if assets == nil {
		assets = []map[string]interface{}{}
	}
	if automaticTLS == nil {
		automaticTLS = []map[string]interface{}{}
	}
	response := map[string]interface{}{
		// certificates remains an asset-only compatibility alias. Automatic TLS
		// observations deliberately do not participate in certificate CRUD.
		"certificates":        assets,
		"assets":              assets,
		"automatic_tls":       automaticTLS,
		"count":               len(assets),
		"asset_count":         len(assets),
		"automatic_tls_count": len(automaticTLS),
	}
	if len(observationWarnings) > 0 {
		response["observation_warning"] = strings.Join(observationWarnings, "; ")
	}
	writeJSON(w, http.StatusOK, response)
}

// AdminListAutoCertificates handles GET /api/admin/v1/certificates/auto
// Returns only provider auto-issued certificates discovered from Caddy's cert store.
func (h *Handlers) AdminListAutoCertificates(w http.ResponseWriter, r *http.Request) {
	var certs []certstore.DiscoveredCert
	var warnings []string
	for _, observer := range h.TLSObservers {
		observed, err := observer.Observe(r.Context())
		if err != nil {
			warnings = append(warnings, observer.ExecutorID()+": "+err.Error())
		}
		certs = append(certs, observed...)
	}
	if certs == nil {
		certs = []certstore.DiscoveredCert{}
	}
	response := map[string]interface{}{"certificates": certs, "count": len(certs)}
	if len(warnings) > 0 {
		response["observation_warning"] = strings.Join(warnings, "; ")
	}
	writeJSON(w, http.StatusOK, response)
}

// AdminUploadCertificate handles POST /api/admin/v1/certificates
// Accepts JSON body with cert_pem + key_pem fields (text paste, the common case).
func (h *Handlers) AdminUploadCertificate(w http.ResponseWriter, r *http.Request) {
	if h.CertStore == nil {
		writeError(w, http.StatusNotImplemented, "certificate store not available")
		return
	}

	// Try JSON body first (text paste)
	var body struct {
		CertPEM string `json:"cert_pem"`
		KeyPEM  string `json:"key_pem"`
		Note    string `json:"note"`
		Source  string `json:"source"`
	}
	if err := decodeJSON(r, &body); err == nil && body.CertPEM != "" {
		if body.Source != "" && body.Source != certstore.SourceManualUpload && body.Source != certstore.SourceExternal {
			writeError(w, http.StatusBadRequest, "source must be manual_upload or external")
			return
		}
		cert, err := h.CertStore.Upload(certstore.UploadRequest{
			CertPEM: []byte(body.CertPEM),
			KeyPEM:  []byte(body.KeyPEM),
			Source:  body.Source,
			Note:    body.Note,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, cert)
		return
	}

	// Fallback: multipart form upload
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "provide cert_pem and key_pem as JSON, or multipart cert_file + key_file")
		return
	}

	certFile, _, err := r.FormFile("cert_file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing cert_pem (JSON) or cert_file (multipart)")
		return
	}
	defer certFile.Close()

	keyFile, _, err := r.FormFile("key_file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing key_pem (JSON) or key_file (multipart)")
		return
	}
	defer keyFile.Close()

	certPEM, _ := io.ReadAll(io.LimitReader(certFile, 1<<19))
	keyPEM, _ := io.ReadAll(io.LimitReader(keyFile, 1<<19))
	note := r.FormValue("note")
	source := r.FormValue("source")
	if source != "" && source != certstore.SourceManualUpload && source != certstore.SourceExternal {
		writeError(w, http.StatusBadRequest, "source must be manual_upload or external")
		return
	}

	cert, err := h.CertStore.Upload(certstore.UploadRequest{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		Source:  source,
		Note:    note,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, cert)
}

// AdminDeleteCertificate handles DELETE /api/admin/v1/certificates/{id}
func (h *Handlers) AdminDeleteCertificate(w http.ResponseWriter, r *http.Request) {
	if h.CertStore == nil {
		writeError(w, http.StatusNotImplemented, "certificate store not available")
		return
	}
	if h.TLSLifecycle == nil {
		// WHY: deletion policy, reference checks, and race handling must have one
		// implementation. A wiring failure must fail closed, not revive legacy semantics.
		writeError(w, http.StatusNotImplemented, "TLS lifecycle service not available")
		return
	}
	id := r.PathValue("id")
	preview, err := h.TLSLifecycle.DeleteCertificate(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, tlslifecycle.ErrCertificateNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	if !preview.Allowed {
		status := http.StatusConflict
		if preview.ReasonCode == tlslifecycle.ReasonManagedByProvider {
			status = http.StatusForbidden
		}
		writeJSON(w, status, preview)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "id": id, "impact": preview})
}

func (h *Handlers) AdminPreviewDeleteCertificate(w http.ResponseWriter, r *http.Request) {
	if h.TLSLifecycle == nil {
		writeError(w, http.StatusNotImplemented, "TLS lifecycle service not available")
		return
	}
	preview, err := h.TLSLifecycle.PreviewDeleteCertificate(r.Context(), r.PathValue("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, tlslifecycle.ErrCertificateNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

// ─── ACME handlers ───

// AdminACMEObtain handles POST /api/admin/v1/acme/obtain
func (h *Handlers) AdminPreviewCertificateBindings(w http.ResponseWriter, r *http.Request) {
	if h.TLSLifecycle == nil {
		writeError(w, http.StatusNotImplemented, "TLS lifecycle service not available")
		return
	}
	preview, err := h.TLSLifecycle.PreviewCertificateBindings(r.Context(), r.PathValue("id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, tlslifecycle.ErrCertificateNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *Handlers) AdminBindCertificateRoutes(w http.ResponseWriter, r *http.Request) {
	if h.TLSLifecycle == nil {
		writeError(w, http.StatusNotImplemented, "TLS lifecycle service not available")
		return
	}
	var input struct {
		RouteIDs []string `json:"route_ids"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := h.TLSLifecycle.BindCertificateToRoutes(r.Context(), r.PathValue("id"), input.RouteIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.PendingState != nil {
		_ = h.PendingState.MarkPending("certificate bound to routes: " + r.PathValue("id"))
	}
	status := http.StatusOK
	response := map[string]interface{}{"status": "updated", "binding": preview}
	if h.Apply != nil {
		if _, err := h.Apply.TryApply(r.Context()); err != nil {
			status = http.StatusAccepted
			response["status"] = "pending_apply"
			response["warning"] = err.Error()
		}
	}
	writeJSON(w, status, response)
}

func (h *Handlers) AdminACMEObtain(w http.ResponseWriter, r *http.Request) {
	if h.ACMEClient == nil || !h.ACMEClient.Available() {
		writeError(w, http.StatusNotImplemented, "ACME not available — configure proxy.email in settings")
		return
	}
	var body struct {
		Domains []string `json:"domains"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if len(body.Domains) == 0 {
		writeError(w, http.StatusBadRequest, "at least one domain required")
		return
	}
	for i := range body.Domains {
		body.Domains[i] = strings.ToLower(strings.TrimSpace(body.Domains[i]))
		if body.Domains[i] == "" {
			writeError(w, http.StatusBadRequest, "domains must not be empty")
			return
		}
		if strings.HasPrefix(body.Domains[i], "*.") {
			writeError(w, http.StatusBadRequest, "wildcard certificates require DNS-01 and must be uploaded as certificate assets")
			return
		}
	}
	if err := h.prepareACMEHTTP01(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "prepare ACME HTTP-01 route: "+err.Error())
		return
	}
	result, err := h.ACMEClient.Obtain(r.Context(), body.Domains)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"cert_id": result.CertID,
		"status":  "issued",
	})
}

// ACMEHTTPChallenge serves only active lego tokens. Caddy exposes this path on
// port 80; ACME validators cannot use admin authentication.
func (h *Handlers) ACMEHTTPChallenge(w http.ResponseWriter, r *http.Request) {
	if h.ACMEClient == nil {
		http.NotFound(w, r)
		return
	}
	value, ok := h.ACMEClient.HTTPChallengeResponse(r.PathValue("token"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, value)
}

func (h *Handlers) prepareACMEHTTP01(ctx context.Context) error {
	if h.Apply != nil {
		_, err := h.Apply.ForceApply(ctx)
		return err
	}
	return fmt.Errorf("apply service is unavailable")
}

// AdminACMEStatus handles GET /api/admin/v1/acme/status
func (h *Handlers) AdminACMEStatus(w http.ResponseWriter, r *http.Request) {
	email := ""
	if h.Config != nil {
		email = h.Config.Proxy.Email
	}
	status := tool.DetectACME(email)
	if h.ACMEClient == nil || !h.ACMEClient.Available() {
		status.Available = false
		status.Message = "ACME client is unavailable; check account-key storage and server logs"
	}
	writeJSON(w, http.StatusOK, status)
}

// ─── Certificate Renewal ───

// AdminCheckCertExpiry handles GET /api/admin/v1/certificates/expiry
func (h *Handlers) AdminCheckCertExpiry(w http.ResponseWriter, r *http.Request) {
	if h.CertStore == nil {
		writeError(w, http.StatusNotImplemented, "certificate store not available")
		return
	}
	var acmeRenewer certstore.ACMERenewer
	if h.ACMEClient != nil && h.ACMEClient.Available() {
		acmeRenewer = h.ACMEClient
	}
	checker := certstore.NewCertRenewalChecker(h.CertStore, acmeRenewer)
	certs, err := checker.Check(r.Context(), 90)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if certs == nil {
		certs = []certstore.ExpiringCert{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"expiring": certs,
		"count":    len(certs),
	})
}

// AdminRenewCert handles POST /api/admin/v1/certificates/{id}/renew
func (h *Handlers) AdminRenewCert(w http.ResponseWriter, r *http.Request) {
	if h.CertStore == nil {
		writeError(w, http.StatusNotImplemented, "certificate store not available")
		return
	}
	id := r.PathValue("id")
	cert, err := h.CertStore.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cert == nil {
		writeError(w, http.StatusNotFound, "certificate not found")
		return
	}
	if cert.Source == certstore.SourceLocalACME {
		if err := h.prepareACMEHTTP01(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "prepare ACME HTTP-01 route: "+err.Error())
			return
		}
	}
	var acmeRenewer certstore.ACMERenewer
	if h.ACMEClient != nil && h.ACMEClient.Available() {
		acmeRenewer = h.ACMEClient
	}
	checker := certstore.NewCertRenewalChecker(h.CertStore, acmeRenewer)
	if h.Apply != nil {
		checker.SetRenewalCoordinator(h.Apply)
	} else {
		checker.SetProviderReloader(h.ProvReg)
	}
	result, err := checker.Renew(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := http.StatusOK
	if result.PendingApply {
		status = http.StatusAccepted
		if h.PendingState != nil {
			_ = h.PendingState.MarkPending("certificate renewed but provider reload is pending: " + id)
		}
	}
	writeJSON(w, status, result)
}

// AdminInfraStatus returns all infrastructure dependencies status.
// GET /api/admin/v1/infra/status
func (h *Handlers) AdminInfraStatus(w http.ResponseWriter, r *http.Request) {
	email := ""
	if h.Config != nil {
		email = h.Config.Proxy.Email
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": []tool.Status{
			tool.DetectACME(email),
			tool.DetectIPTables(),
			tool.DetectDNSMasq(),
		},
	})
}
