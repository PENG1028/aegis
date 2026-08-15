package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"aegis/internal/action"
	"aegis/internal/adminauth"
)

// TestProxyHandlerPassesCallerIdentity is a regression test: the cross-node
// proxy executed the target handler on a bare mux with NO caller identity —
// the target handler saw an empty action context and no admin context, so
// audit logs recorded empty actors and identity-dependent handlers behaved
// inconsistently. The caller's identity (admin user / token type / space)
// must travel with the proxied request and be re-injected on the target.
func TestProxyHandlerPassesCallerIdentity(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/admin/v1/whoami", func(w http.ResponseWriter, r *http.Request) {
		ac := action.GetActionContext(r.Context())
		adm := adminauth.GetAdminContext(r.Context())
		tokenType, spaceID := "", ""
		if ac != nil {
			tokenType = ac.TokenType
			spaceID = ac.SpaceID
		}
		adminID := ""
		if adm != nil {
			adminID = adm.UserID
		}
		json.NewEncoder(w).Encode(map[string]string{
			"token_type": tokenType, "space_id": spaceID, "admin_id": adminID,
		})
	})

	h := &Handlers{proxyMux: mux}
	handler := newProxyHandler(h)

	args := json.RawMessage(`{
		"method":"GET","path":"/api/admin/v1/whoami",
		"headers":{
			"X-Aegis-Proxy-Admin-ID":"user_1",
			"X-Aegis-Proxy-Token-Type":"admin",
			"X-Aegis-Proxy-Space-ID":""
		}
	}`)
	respRaw, err := handler(context.Background(), "node_a", args)
	if err != nil {
		t.Fatal(err)
	}
	resp := respRaw.(ProxyResponse)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body: %s", resp.StatusCode, string(resp.Body))
	}
	var got map[string]string
	if err := json.Unmarshal(resp.Body, &got); err != nil {
		t.Fatal(err)
	}
	if got["token_type"] != "admin" {
		t.Fatalf("target handler saw token_type=%q, want admin (identity not propagated): %s", got["token_type"], string(resp.Body))
	}
	if got["admin_id"] != "user_1" {
		t.Fatalf("target handler saw admin_id=%q, want user_1", got["admin_id"])
	}
}

// TestProxyHandlerSpaceIdentity propagates space-scoped identity too.
func TestProxyHandlerSpaceIdentity(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/who", func(w http.ResponseWriter, r *http.Request) {
		ac := action.GetActionContext(r.Context())
		tokenType, spaceID := "", ""
		if ac != nil {
			tokenType = ac.TokenType
			spaceID = ac.SpaceID
		}
		json.NewEncoder(w).Encode(map[string]string{"token_type": tokenType, "space_id": spaceID})
	})
	h := &Handlers{proxyMux: mux}
	handler := newProxyHandler(h)

	args := json.RawMessage(`{
		"method":"GET","path":"/api/v1/who",
		"headers":{"X-Aegis-Proxy-Token-Type":"service","X-Aegis-Proxy-Space-ID":"space_a"}
	}`)
	respRaw, err := handler(context.Background(), "node_b", args)
	if err != nil {
		t.Fatal(err)
	}
	resp := respRaw.(ProxyResponse)
	if !strings.Contains(string(resp.Body), `"space_id":"space_a"`) {
		t.Fatalf("space identity not propagated: %s", string(resp.Body))
	}
	if !strings.Contains(string(resp.Body), `"token_type":"service"`) {
		t.Fatalf("token type not propagated: %s", string(resp.Body))
	}
}
