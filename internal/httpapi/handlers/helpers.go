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

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
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
