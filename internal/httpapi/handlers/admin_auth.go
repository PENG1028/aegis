package handlers

import (
	"net/http"
	"time"

	"aegis/internal/adminauth"
)

// AdminLogin handles POST /api/admin/v1/auth/login
func (h *Handlers) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.Username == "" || input.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	ip := r.RemoteAddr
	userAgent := r.Header.Get("User-Agent")

	result, err := h.AdminAuth.Login(input.Username, input.Password, ip, userAgent)
	if err != nil {
		// Check for rate limiting
		errMsg := err.Error()
		if len(errMsg) > 20 && errMsg[:7] == "too many" {
			writeError(w, http.StatusTooManyRequests, errMsg)
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Session cookie Secure flag: true only when the request arrived over TLS.
	// Dynamic detection avoids the bug where Secure=true on HTTP breaks login
	// (browser refuses to send Secure cookies over plaintext).
	//
	// When Caddy terminates TLS, it sets X-Forwarded-Proto: https so Aegis
	// (which listens on 127.0.0.1:7380 without TLS) knows the original request
	// was encrypted end-to-end from the browser to Caddy.
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	adminauth.SetSessionCookie(w, result.SessionToken, result.ExpiresAt.Format(time.RFC3339), isSecure)

	resp := map[string]interface{}{
		"user": map[string]interface{}{
			"id":       result.User.ID,
			"username": result.User.Username,
		},
		"expires_at": result.ExpiresAt.Format(time.RFC3339),
	}

	// Warn if login happened over plain HTTP — credentials were exposed.
	if !isSecure {
		resp["warning"] = "Credentials sent over HTTP (not encrypted). Configure a domain in Settings to enable HTTPS."
	}

	writeJSON(w, http.StatusOK, resp)
}

// AdminLogout handles POST /api/admin/v1/auth/logout
func (h *Handlers) AdminLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("aegis_admin_session")
	if err == nil && cookie.Value != "" {
		ip := r.RemoteAddr
		userAgent := r.Header.Get("User-Agent")
		_ = h.AdminAuth.Logout(adminauth.HashSessionToken(cookie.Value), ip, userAgent)
	}
	adminauth.ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "logged out",
	})
}

// AdminChangePassword handles POST /api/admin/v1/auth/change-password
func (h *Handlers) AdminChangePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if input.CurrentPassword == "" || input.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	// Get user from session context
	ac := adminauth.GetAdminContext(r.Context())
	if ac == nil {
		writeError(w, http.StatusUnauthorized, "admin session required")
		return
	}

	if err := h.AdminAuth.ChangePassword(ac.UserID, input.CurrentPassword, input.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "password changed successfully",
	})
}

// AdminMe handles GET /api/admin/v1/auth/me
func (h *Handlers) AdminMe(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("aegis_admin_session")
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "session required")
		return
	}

	user, err := h.AdminAuth.ValidateSession(adminauth.HashSessionToken(cookie.Value))
	if err != nil || user == nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}
