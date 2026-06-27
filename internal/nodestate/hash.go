package nodestate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// ComputeStateHash computes a stable SHA-256 hash of state JSON.
// It normalizes the JSON by re-marshaling through a map to ensure
// consistent key ordering. Note: this is best-effort canonicalization.
// If the JSON contains nested objects, the map-based normalization
// produces stable output because Go map iteration is randomized but
// json.Marshal of a map[string]interface{} sorts keys alphabetically.
func ComputeStateHash(stateJSON string) (string, error) {
	// Parse as generic JSON to normalize
	var data interface{}
	if err := json.Unmarshal([]byte(stateJSON), &data); err != nil {
		return "", err
	}

	// Re-marshal with sorted keys
	normalized, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	h := sha256.Sum256(normalized)
	return hex.EncodeToString(h[:]), nil
}

// NormalizeJSON sorts all keys recursively for stable comparison.
func NormalizeJSON(raw string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return "", err
	}
	data = normalizeValue(data)
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		// Sort keys
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		m := make(map[string]interface{}, len(val))
		for _, k := range keys {
			m[k] = normalizeValue(val[k])
		}
		return m
	case []interface{}:
		for i, item := range val {
			val[i] = normalizeValue(item)
		}
		return val
	default:
		return val
	}
}

// MustComputeHash computes a hash or returns empty string on error.
func MustComputeHash(stateJSON string) string {
	h, err := ComputeStateHash(stateJSON)
	if err != nil {
		return ""
	}
	return h
}
