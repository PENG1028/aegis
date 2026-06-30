package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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
	ct := r.Header.Get("Content-Type")
	if ct != "" && ct != "application/json" && !strings.HasPrefix(ct, "application/json;") {
		return fmt.Errorf("unsupported Content-Type: %s (expected application/json)", ct)
	}
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

// ─── Pagination ───

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// paginationMeta holds pagination metadata for list responses.
type paginationMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// paginationParams parses limit and offset from query string.
// Defaults: limit=50, offset=0. Max limit: 200.
func paginationParams(r *http.Request) (limit, offset int) {
	limit = DefaultLimit
	offset = 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
			if limit > MaxLimit {
				limit = MaxLimit
			}
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

// paginateSlice applies limit/offset to a slice and returns the page.
func paginateSlice[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

// writePaginatedJSON writes a paginated list response.
// Uses the standard envelope: {"data": [...], "meta": {"total": N, "limit": L, "offset": O}}
func writePaginatedJSON(w http.ResponseWriter, status int, data interface{}, total, limit, offset int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": data,
		"meta": paginationMeta{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	})
}
