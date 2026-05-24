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
	if cfg.Image.DefaultJPEGQuality != 90 {
		t.Errorf("image.default_jpeg_quality = %d, want 90", cfg.Image.DefaultJPEGQuality)
	}
	if cfg.PDF.DefaultDPI != 150 {
		t.Errorf("pdf.default_dpi = %d, want 150", cfg.PDF.DefaultDPI)
	}
}

func TestConfigPatchPersistsImageQuality(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	ts := newTestServer(t, t.TempDir())

	resp := patchConfig(t, ts.URL+"/v1/config", `{"image":{"default_jpeg_quality":75}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got config.Config
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Image.DefaultJPEGQuality != 75 {
		t.Errorf("response image.default_jpeg_quality = %d, want 75", got.Image.DefaultJPEGQuality)
	}
	// Unmentioned fields keep their defaults — this is a deep merge, not a replace.
	if got.PDF.DefaultDPI != 150 {
		t.Errorf("pdf.default_dpi = %d, want 150 (untouched)", got.PDF.DefaultDPI)
	}

	// And it persisted to disk.
	reloaded, _, err := config.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Image.DefaultJPEGQuality != 75 {
		t.Errorf("persisted image.default_jpeg_quality = %d, want 75", reloaded.Image.DefaultJPEGQuality)
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
