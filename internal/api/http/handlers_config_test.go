package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/config"
)

// patchConfig sends a PATCH /v1/config request with the given JSON body.
func patchConfig(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	return resp
}

func TestConfigGetReturnsDefaults(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	ts := newTestServer(t, t.TempDir())

	resp, err := http.Get(ts.URL + "/v1/config")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var cfg config.Config
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Theme.Name != "forge" {
		t.Errorf("theme.name = %q, want forge", cfg.Theme.Name)
	}
	if cfg.Image.DefaultJPEGQuality != 90 {
		t.Errorf("image.default_jpeg_quality = %d, want 90", cfg.Image.DefaultJPEGQuality)
	}
}

func TestConfigPatchPersistsTheme(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	ts := newTestServer(t, t.TempDir())

	resp := patchConfig(t, ts.URL+"/v1/config", `{"theme":{"name":"snow"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got config.Config
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Theme.Name != "snow" {
		t.Errorf("response theme.name = %q, want snow", got.Theme.Name)
	}
	// Unmentioned fields keep their defaults — this is a deep merge, not a replace.
	if got.Image.DefaultJPEGQuality != 90 {
		t.Errorf("image.default_jpeg_quality = %d, want 90 (untouched)", got.Image.DefaultJPEGQuality)
	}

	// And it persisted to disk.
	reloaded, _, err := config.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Theme.Name != "snow" {
		t.Errorf("persisted theme.name = %q, want snow", reloaded.Theme.Name)
	}
}

func TestConfigPatchRejectsServerWrites(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	ts := newTestServer(t, t.TempDir())

	resp := patchConfig(t, ts.URL+"/v1/config", `{"server":{"allow_roots":["/etc"]}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (server.* is not writable)", resp.StatusCode)
	}
}

func TestConfigPatchRejectsInvalidTheme(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	ts := newTestServer(t, t.TempDir())

	resp := patchConfig(t, ts.URL+"/v1/config", `{"theme":{"name":"galaxy"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (unknown theme)", resp.StatusCode)
	}
}

func TestConfigPatchRejectsInvalidQuality(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	ts := newTestServer(t, t.TempDir())

	resp := patchConfig(t, ts.URL+"/v1/config", `{"image":{"default_jpeg_quality":250}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (quality out of range)", resp.StatusCode)
	}
}

func TestConfigPatchRejectsPathOutsideRoots(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	ts := newTestServer(t, t.TempDir())

	// /etc/... is not under the server's temp-dir allow-root.
	resp := patchConfig(t, ts.URL+"/v1/config", `{"archive":{"default_destination":"/etc/handy-out"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (path outside allow-roots)", resp.StatusCode)
	}
}
