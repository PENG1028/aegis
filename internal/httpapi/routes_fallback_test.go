package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnknownAPIRouteDoesNotFallThroughToSPA(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, &Services{})

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "unknown endpoint", method: http.MethodGet, path: "/api/admin/v1/not-real"},
		{name: "unsupported method", method: http.MethodPatch, path: "/api/admin/v1/routes/rt_test/tls-binding"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
				t.Fatalf("content type = %q", contentType)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode JSON response: %v", err)
			}
			if body.Error.Code != "NOT_FOUND" {
				t.Fatalf("error code = %q", body.Error.Code)
			}
		})
	}
}
