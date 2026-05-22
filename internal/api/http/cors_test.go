package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/server"
)

func TestCORSPreflight(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/v1/uploads", nil)
	req.Header.Set("Origin", "chrome-extension://abcdefghijklmnop")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status: got %d want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "chrome-extension://abcdefghijklmnop" {
		t.Errorf("Allow-Origin: got %q, want the echoed extension origin", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got == "" {
		t.Errorf("Allow-Methods header missing on preflight")
	}
}

func TestCORSAllowedOriginEchoed(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin: got %q want http://localhost:5173", got)
	}
	if got := resp.Header.Get("Vary"); got != "Origin" {
		t.Errorf("Vary: got %q want Origin", got)
	}
}

func TestCORSDeniedOriginGetsNoHeaders(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	// The request is still served — CORS only governs what the browser
	// exposes to the page — but no Allow-Origin header is emitted.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin: got %q, want empty for a denied origin", got)
	}
}

func TestCORSNoOriginHeader(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp, err := http.Get(ts.URL + "/v1/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin set on a non-CORS request: %q", got)
	}
}

func TestCORSExplicitOriginsReplaceDefaults(t *testing.T) {
	s := New(
		server.Options{
			AllowRoots:  []string{t.TempDir()},
			CORSOrigins: []string{"https://tools.example"},
		},
		queue.New(), nil,
	)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	// The configured origin is allowed.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/health", nil)
	req.Header.Set("Origin", "https://tools.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://tools.example" {
		t.Errorf("configured origin not allowed: got %q", got)
	}

	// localhost — a built-in default — is now denied, because a non-empty
	// list replaces the defaults.
	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/health", nil)
	req2.Header.Set("Origin", "http://localhost:5173")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp2.Body.Close()
	if got := resp2.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("localhost still allowed despite explicit cors_origins: %q", got)
	}
}

func TestOriginAllowed(t *testing.T) {
	allowed := defaultCORSOrigins
	cases := map[string]bool{
		"http://localhost":                    true,
		"http://localhost:5173":               true,
		"http://127.0.0.1:8080":               true,
		"chrome-extension://abcdefghijklmnop": true,
		"http://localhost.evil.com":           false,
		"https://localhost:5173":              false,
		"https://evil.example.com":            false,
		"":                                    false,
	}
	for origin, want := range cases {
		if got := originAllowed(origin, allowed); got != want {
			t.Errorf("originAllowed(%q) = %v, want %v", origin, got, want)
		}
	}
}
