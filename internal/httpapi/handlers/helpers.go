package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// apiError is the unified API error response format.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errorCodeFromStatus maps HTTP status codes to machine-readable error codes.
func errorCodeFromStatus(status int) string {
	switch {
	case status == 400:
		return "BAD_REQUEST"
	case status == 401:
		return "UNAUTHORIZED"
	case status == 403:
		return "FORBIDDEN"
	case status == 404:
		return "NOT_FOUND"
	case status == 409:
		return "CONFLICT"
	case status == 429:
		return "RATE_LIMITED"
	case status >= 500:
		return "INTERNAL_ERROR"
	default:
		return "UNKNOWN"
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]apiError{
		"error": {Code: errorCodeFromStatus(status), Message: msg},
	})
}

func writeErrorCode(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]apiError{
		"error": {Code: code, Message: msg},
	})
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	// 1 MB limit — prevents memory exhaustion from oversized payloads
	const maxBodySize = 1 << 20
	limitedBody := io.LimitReader(r.Body, maxBodySize)
	if err := json.NewDecoder(limitedBody).Decode(v); err != nil {
		return err
	}
	// Drain any remaining bytes so the connection can be reused.
	// If the body was truncated, return an error.
	if n, _ := io.Copy(io.Discard, r.Body); n > 0 {
		return fmt.Errorf("request body exceeds %d byte limit", maxBodySize)
	}
	return nil
}
