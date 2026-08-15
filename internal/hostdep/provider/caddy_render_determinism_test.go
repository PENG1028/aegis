package provider

import (
	"bytes"
	"strings"
	"testing"
)

// TestWriteReverseProxyHeadersDeterministic is a regression test: header
// injection iterated the headers map, so the rendered Caddyfile bytes were
// non-deterministic across runs. The apply layer compares rendered-config
// hashes to skip no-op applies — unstable bytes defeat that mechanism and
// cause an unnecessary reload on every apply.
func TestWriteReverseProxyHeadersDeterministic(t *testing.T) {
	headers := map[string]string{
		"X-Auth-Token":  "abc",
		"X-Caller":      "svc-a",
		"X-Trace":       "t1",
		"X-Gateway-ID":  "gw1",
		"X-Upstream":    "up1",
		"X-Correlation": "c1",
	}
	upstream := "http://127.0.0.1:8080"

	render := func() string {
		var buf bytes.Buffer
		writeReverseProxy(&buf, upstream, headers, "    ")
		return buf.String()
	}

	first := render()
	for i := 0; i < 10; i++ {
		if got := render(); got != first {
			t.Fatalf("rendered output differs between runs (map iteration order) — apply hash-skip is broken:\n---\n%s\n---\n%s", first, got)
		}
	}
	if !strings.Contains(first, "header_up X-Auth-Token") {
		t.Fatalf("header not rendered: %s", first)
	}
}
