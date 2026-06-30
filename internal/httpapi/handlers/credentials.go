package handlers

import (
	"encoding/json"
	"net/http"

	"aegis/internal/credential"
)

// CredentialHandlers holds the credential service for HTTP handlers.
type CredentialHandlers struct {
	Svc *credential.Service
}

// ListCredentials handles GET /api/admin/v1/credentials
func (ch *CredentialHandlers) ListCredentials(w http.ResponseWriter, r *http.Request) {
	list, err := ch.Svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list credentials: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"credentials": list,
		"count":       len(list),
	})
}

// CreateCredential handles POST /api/admin/v1/credentials
func (ch *CredentialHandlers) CreateCredential(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Alias       string `json:"alias"`
		ConnString  string `json:"conn_string"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Alias == "" || req.ConnString == "" {
		writeError(w, http.StatusBadRequest, "alias and conn_string are required")
		return
	}

	result, err := ch.Svc.Create(r.Context(), req.Alias, req.ConnString, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"credential": result.Credential,
	})
}

// GetCredential handles GET /api/admin/v1/credentials/{id}
func (ch *CredentialHandlers) GetCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := ch.Svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if c == nil {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	// Never expose encrypted fields
	c.EncryptedConnString = ""
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"credential": c,
	})
}

// DeleteCredential handles DELETE /api/admin/v1/credentials/{id}
func (ch *CredentialHandlers) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ch.Svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"message": "credential deleted",
	})
}

// RotateCredential handles POST /api/admin/v1/credentials/{id}/rotate
func (ch *CredentialHandlers) RotateCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		ConnString string `json:"conn_string"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ConnString == "" {
		writeError(w, http.StatusBadRequest, "conn_string is required")
		return
	}

	c, rawToken, err := ch.Svc.Rotate(r.Context(), id, req.ConnString)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.EncryptedConnString = ""
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"credential": c,
		"raw_token":  rawToken, // returned once
	})
}

// ResolveByAlias handles GET /api/admin/v1/credentials/resolve?alias=xxx
// Decrypts and returns the parsed connection info (host, port, user, database, scheme).
// Used by external tools (dbmanage, etc.) to resolve an alias to real connection parameters.
func (ch *CredentialHandlers) ResolveByAlias(w http.ResponseWriter, r *http.Request) {
	alias := r.URL.Query().Get("alias")
	if alias == "" {
		writeError(w, http.StatusBadRequest, "alias query parameter is required")
		return
	}

	info, err := ch.Svc.DecryptAndResolve(r.Context(), alias)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alias":    alias,
		"scheme":   info.Scheme,
		"host":     info.Host,
		"port":     info.Port,
		"user":     info.User,
		"password": info.Password,
		"database": info.Database,
		"raw_query": info.RawQuery,
		"target":   info.TargetAddr(),
	})
}

// RevealCredential handles POST /api/admin/v1/credentials/{id}/reveal
// Returns the raw connection string ONCE. Audit-logged.
func (ch *CredentialHandlers) RevealCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := ch.Svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if c == nil {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}

	// Decrypt and return raw connection string once
	raw, err := ch.Svc.RevealRaw(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decrypt: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"credential": map[string]interface{}{
			"id":               c.ID,
			"alias":            c.Alias,
			"secret_version":   c.SecretVersion,
			"raw_conn_string":  raw,
		},
		"warning": "raw connection string revealed — store securely, will not be shown again",
	})
}
