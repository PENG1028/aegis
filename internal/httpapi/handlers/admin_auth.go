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

	// Set session cookie
	adminauth.SetSessionCookie(w, result.SessionToken, result.ExpiresAt.Format(time.RFC3339), h.Config.Server.SessionSecure)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":       result.User.ID,
			"username": result.User.Username,
		},
		"expires_at": result.ExpiresAt.Format(time.RFC3339),
	})
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
