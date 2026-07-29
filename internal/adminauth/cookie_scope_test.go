package adminauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSessionCookiePathCoversNonAdminEndpoints is the regression test for the
// UI 401 bug: the session cookie was scoped to /api/admin/v1, so the browser
// never sent it to admin-only endpoints outside that prefix and the Apply,
// Rollback and Changes pages failed.
//
// Asserted as cookie-Path prefix matching rather than by string equality, so the
// test states the property that matters instead of restating the constant.
func TestSessionCookiePathCoversNonAdminEndpoints(t *testing.T) {
	w := httptest.NewRecorder()
	SetSessionCookie(w, "tok", "", false)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	got := cookies[0]

	// Paths the UI calls that require an admin identity. Sourced from
	// ui/src/lib/real-api-client.ts.
	adminOnlyPaths := []string{
		"/api/apply",
		"/api/apply/dry-run",
		"/api/apply/history",
		"/api/rollback",
		"/api/config/current",
		"/api/config/diff",
		"/api/config/preview",
		"/api/exposures",
		"/api/health",
		"/api/routes/rt_1",
		"/api/services/svc_1",
		"/api/admin/v1/scopes",
	}

	for _, p := range adminOnlyPaths {
		if !cookiePathMatches(got.Path, p) {
			t.Errorf("cookie Path %q is not sent to %q — endpoint will 401", got.Path, p)
		}
	}
}

// cookiePathMatches implements RFC 6265 §5.1.4 path-match: equal, or the cookie
// path is a prefix ending in "/", or the next character in the request path is
// "/". Reimplemented here because net/http exposes no such helper.
func cookiePathMatches(cookiePath, requestPath string) bool {
	if cookiePath == requestPath {
		return true
	}
	if !strings.HasPrefix(requestPath, cookiePath) {
		return false
	}
	if strings.HasSuffix(cookiePath, "/") {
		return true
	}
	return len(requestPath) > len(cookiePath) && requestPath[len(cookiePath)] == '/'
}

// TestClearSessionCookieMatchesSetPath pins the delete-path invariant: a browser
// removes a cookie only when name, Path and Domain match the original, so a
// divergence between setter and clearer would silently break logout.
func TestClearSessionCookieMatchesSetPath(t *testing.T) {
	setRec := httptest.NewRecorder()
	SetSessionCookie(setRec, "tok", "", false)
	clearRec := httptest.NewRecorder()
	ClearSessionCookie(clearRec)

	set := setRec.Result().Cookies()[0]
	clear := clearRec.Result().Cookies()[0]

	if set.Path != clear.Path {
		t.Errorf("Path mismatch: set=%q clear=%q — logout cannot delete the cookie", set.Path, clear.Path)
	}
	if set.Name != clear.Name {
		t.Errorf("Name mismatch: set=%q clear=%q", set.Name, clear.Name)
	}
	if clear.MaxAge >= 0 {
		t.Errorf("expected negative MaxAge to expire the cookie, got %d", clear.MaxAge)
	}
}

// TestSessionCookieHardening pins the attributes that keep the session usable
// only by the app itself.
func TestSessionCookieHardening(t *testing.T) {
	w := httptest.NewRecorder()
	SetSessionCookie(w, "tok", "", true)
	c := w.Result().Cookies()[0]

	if !c.HttpOnly {
		t.Error("HttpOnly must stay set — session must be unreadable from JS")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite must stay Strict, got %v", c.SameSite)
	}
	if !c.Secure {
		t.Error("Secure must follow the secure argument")
	}
}
