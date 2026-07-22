package aegisgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"aegis/internal/serviceauth"
)

type fakeRegistry struct {
	records []serviceauth.ServiceRecord
	err     error
}

func (r fakeRegistry) FindByName(context.Context, string) ([]serviceauth.ServiceRecord, error) {
	return r.records, r.err
}

type fakeLocator struct {
	nodeID string
}

func (l fakeLocator) LocateServiceNode(context.Context, string) (string, error) {
	return l.nodeID, nil
}

type fakeRemoteCaller struct {
	nodeID  string
	request ServiceCallRequest
	headers ServiceCallHeaders
}

func (c *fakeRemoteCaller) CallService(_ context.Context, nodeID string, req ServiceCallRequest, headers ServiceCallHeaders) (*HTTPResponse, error) {
	c.nodeID = nodeID
	c.request = req
	c.headers = headers
	return &HTTPResponse{StatusCode: http.StatusAccepted, Body: json.RawMessage(`{"remote":true}`)}, nil
}

func TestCallServiceForwardsToLocalService(t *testing.T) {
	var gotTicket, gotCaller string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTicket = r.Header.Get("X-Service-Ticket")
		gotCaller = r.Header.Get("X-Caller-Service")
		w.Header().Set("X-Test-Target", "local")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	host, port := mustHostPort(t, target.URL)
	gw := New(Options{Registry: fakeRegistry{records: []serviceauth.ServiceRecord{{
		Name:       "worker",
		Host:       host,
		ListenPort: port,
		Status:     "active",
	}}}})

	resp, err := gw.CallService(context.Background(), ServiceCallRequest{
		Target: "worker",
		Method: http.MethodPost,
		Path:   "/run",
		Body:   json.RawMessage(`{"job":"x"}`),
	}, ServiceCallHeaders{ServiceTicket: "ticket-1", CallerService: "caller"})
	if err != nil {
		t.Fatalf("CallService: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("body = %s", resp.Body)
	}
	if resp.Headers["X-Test-Target"] != "local" {
		t.Fatalf("headers = %+v", resp.Headers)
	}
	if gotTicket != "ticket-1" || gotCaller != "caller" {
		t.Fatalf("forwarded identity headers ticket=%q caller=%q", gotTicket, gotCaller)
	}
}

func TestCallServiceForwardsToRemoteNode(t *testing.T) {
	remote := &fakeRemoteCaller{}
	gw := New(Options{
		Registry:     fakeRegistry{},
		Locator:      fakeLocator{nodeID: "node-b"},
		RemoteCaller: remote,
		SelfNodeID:   "node-a",
	})

	resp, err := gw.CallService(context.Background(), ServiceCallRequest{
		Target: "worker",
		Method: http.MethodGet,
		Path:   "/status",
	}, ServiceCallHeaders{ServiceTicket: "ticket-2", CallerService: "caller"})
	if err != nil {
		t.Fatalf("CallService: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	if remote.nodeID != "node-b" || remote.request.Target != "worker" {
		t.Fatalf("remote call = node %q request %+v", remote.nodeID, remote.request)
	}
	if remote.headers.ServiceTicket != "ticket-2" {
		t.Fatalf("headers = %+v", remote.headers)
	}
}

func TestCallServiceRejectsInvalidRequest(t *testing.T) {
	gw := New(Options{Registry: fakeRegistry{}})
	_, err := gw.CallService(context.Background(), ServiceCallRequest{Target: "worker"}, ServiceCallHeaders{})
	if err == nil {
		t.Fatal("CallService succeeded, want validation error")
	}
	if ErrorStatus(err) != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", ErrorStatus(err), http.StatusBadRequest)
	}
}

func TestCallServiceMissingTargetWithoutLocator(t *testing.T) {
	gw := New(Options{Registry: fakeRegistry{}})
	_, err := gw.CallService(context.Background(), ServiceCallRequest{
		Target: "missing",
		Method: http.MethodGet,
		Path:   "/status",
	}, ServiceCallHeaders{})
	if err == nil {
		t.Fatal("CallService succeeded, want missing target error")
	}
	if ErrorStatus(err) != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", ErrorStatus(err), http.StatusNotFound)
	}
}

func mustHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse test URL %q: %v", rawURL, err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse test URL port %q: %v", rawURL, err)
	}
	return u.Hostname(), port
}
