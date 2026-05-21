package http

import (
	"bytes"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"testing"
)

// TestPreviewEndpoint drives GET /v1/preview: a PNG fixture renders to a PNG
// thumbnail with an ETag, and a conditional re-request returns 304.
func TestPreviewEndpoint(t *testing.T) {
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	ts := newTestServer(t, dir)

	endpoint := ts.URL + "/v1/preview?path=" + url.QueryEscape(src)

	resp, err := http.Get(endpoint)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body %s)", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Fatalf("Content-Type: got %q want image/png", ct)
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("expected an ETag header")
	}
	if _, err := png.Decode(bytes.NewReader(body)); err != nil {
		t.Fatalf("response body is not a valid PNG: %v", err)
	}

	// A conditional re-request with the ETag must short-circuit to 304.
	req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	req.Header.Set("If-None-Match", etag)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("conditional get: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotModified {
		t.Fatalf("conditional status: got %d want 304", resp2.StatusCode)
	}
}

func TestPreviewMissingPath(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp, err := http.Get(ts.URL + "/v1/preview")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

func TestPreviewRejectsPathOutsideRoot(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp, err := http.Get(ts.URL + "/v1/preview?path=" + url.QueryEscape("/etc/hostname"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("path outside allow-root: got %d want 400", resp.StatusCode)
	}
}
