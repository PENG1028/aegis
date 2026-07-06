package handlers

import (
	"io"
	"net/http"

	"aegis/internal/certstore"
	"aegis/internal/infra"
)

// ─── Certificate handlers ───

// AdminListCertificates handles GET /api/admin/v1/certificates
func (h *Handlers) AdminListCertificates(w http.ResponseWriter, r *http.Request) {
	if h.CertStore == nil {
		writeError(w, http.StatusNotImplemented, "certificate store not available")
		return
	}
	certs, err := h.CertStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"certificates": certs, "count": len(certs)})
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
	}
	if err := decodeJSON(r, &body); err == nil && body.CertPEM != "" {
		cert, err := h.CertStore.Upload(certstore.UploadRequest{
			CertPEM: []byte(body.CertPEM),
			KeyPEM:  []byte(body.KeyPEM),
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

	cert, err := h.CertStore.Upload(certstore.UploadRequest{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
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
	id := r.PathValue("id")
	if err := h.CertStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

// ─── ACME handlers ───

// AdminACMEObtain handles POST /api/admin/v1/acme/obtain
func (h *Handlers) AdminACMEObtain(w http.ResponseWriter, r *http.Request) {
	if h.ACMEMgr == nil {
		writeError(w, http.StatusNotImplemented, "ACME not available — configure proxy.email and install certbot")
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

	certID, err := h.ACMEMgr.Obtain(r.Context(), infra.ObtainRequest{Domains: body.Domains})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"cert_id": certID,
		"status":  "issued",
	})
}

// AdminACMEStatus handles GET /api/admin/v1/acme/status
func (h *Handlers) AdminACMEStatus(w http.ResponseWriter, r *http.Request) {
	email := ""
	if h.Config != nil {
		email = h.Config.Proxy.Email
	}
	writeJSON(w, http.StatusOK, infra.DetectCertbot(email))
}

// AdminInfraStatus returns all infrastructure dependencies status.
// GET /api/admin/v1/infra/status
func (h *Handlers) AdminInfraStatus(w http.ResponseWriter, r *http.Request) {
	email := ""
	if h.Config != nil {
		email = h.Config.Proxy.Email
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": []infra.Status{
			infra.DetectCertbot(email),
			infra.DetectIPTables(),
			infra.DetectDNSMasq(),
		},
	})
}
