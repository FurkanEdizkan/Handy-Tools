package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// postAs POSTs body to url with the given Content-Type ("" = none) and
// returns the status code and response body.
func postAs(t *testing.T, url, contentType string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

// terminalOf posts a job request and returns the terminal SSE event.
func terminalOf(t *testing.T, base, path string, req any) progressEvent {
	t.Helper()
	body, _ := json.Marshal(req)
	code, out := postAs(t, base+path, "application/json", body)
	if code != http.StatusAccepted {
		t.Fatalf("POST %s: status %d want 202; body=%s", path, code, out)
	}
	var jr jobResponse
	if err := json.Unmarshal(out, &jr); err != nil || jr.JobID == "" {
		t.Fatalf("POST %s: no job id: %s", path, out)
	}
	events := readSSE(t, base+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	if len(events) == 0 || !events[len(events)-1].Completed {
		t.Fatalf("POST %s: no terminal event in %+v", path, events)
	}
	return events[len(events)-1]
}

// TestImageConvertOutputFileOutsideAllowRoots: output.file must be sandboxed
// like the source — a job may not write outside the allow-roots.
func TestImageConvertOutputFileOutsideAllowRoots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	src := writeTinyPNG(t, root)
	ts := newTestServer(t, root)
	target := filepath.Join(outside, "escaped.png")

	last := terminalOf(t, ts.URL, "/v1/image/convert", convertRequest{
		Source:       fileRef{Path: src},
		TargetFormat: "PNG",
		Output:       outputRef{File: target, Overwrite: true},
	})
	if last.Error == nil || last.Error.Code != tools.CodeBadRequest {
		t.Fatalf("terminal error = %+v, want %s", last.Error, tools.CodeBadRequest)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("file written outside the allow-root (stat err=%v)", err)
	}
}

// TestRenameRunTargetOutsideAllowRoots: a replacement that climbs out of the
// source directory must not move the file outside the allow-roots, and the
// dry-run must refuse to present such a plan.
func TestRenameRunTargetOutsideAllowRoots(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{root, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	victim := writeTextFile(t, root, "victim.txt", "x")
	ts := newTestServer(t, root)
	req := renameRequest{
		Sources: []fileRef{{Path: victim}},
		Pattern: `^victim\.txt$`,
		Replace: "../outside/moved.txt",
	}

	body, _ := json.Marshal(req)
	if code, out := postAs(t, ts.URL+"/v1/rename/inspect", "application/json", body); code != http.StatusBadRequest {
		t.Fatalf("rename/inspect: status %d want 400; body=%s", code, out)
	}

	last := terminalOf(t, ts.URL, "/v1/rename/run", req)
	if last.Error == nil {
		t.Fatalf("rename/run: terminal event %+v, want an error", last)
	}
	if _, err := os.Stat(filepath.Join(outside, "moved.txt")); !os.IsNotExist(err) {
		t.Fatalf("file moved outside the allow-root (stat err=%v)", err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("source should be untouched: %v", err)
	}
}

// TestSandboxRejectionCodeIsConsistent: the same out-of-root path reports
// BAD_REQUEST whether the endpoint is synchronous (400) or an async job
// (terminal SSE error) — not IO_ERROR on one and BAD_REQUEST on the other.
func TestSandboxRejectionCodeIsConsistent(t *testing.T) {
	root := t.TempDir()
	src := writeTinyPNG(t, t.TempDir()) // outside root
	ts := newTestServer(t, root)

	last := terminalOf(t, ts.URL, "/v1/image/convert", convertRequest{
		Source: fileRef{Path: src}, TargetFormat: "PNG", Output: outputRef{Directory: root},
	})
	if last.Error == nil || last.Error.Code != tools.CodeBadRequest {
		t.Fatalf("async rejection = %+v, want %s", last.Error, tools.CodeBadRequest)
	}

	body, _ := json.Marshal(inspectRequest{Source: fileRef{Path: src}})
	code, out := postAs(t, ts.URL+"/v1/archive/inspect", "application/json", body)
	var env map[string]errorEnvelope
	_ = json.Unmarshal(out, &env)
	if code != http.StatusBadRequest || env["error"].Code != tools.CodeBadRequest {
		t.Fatalf("sync rejection: status %d body %s, want 400 %s", code, out, tools.CodeBadRequest)
	}
}

// TestArchiveInspectMissingFileIsNotFound: a missing (in-root) source maps to
// NOT_FOUND/404, matching the gRPC transport's NotFound.
func TestArchiveInspectMissingFileIsNotFound(t *testing.T) {
	root := t.TempDir()
	ts := newTestServer(t, root)
	body, _ := json.Marshal(inspectRequest{Source: fileRef{Path: filepath.Join(root, "missing.zip")}})
	code, out := postAs(t, ts.URL+"/v1/archive/inspect", "application/json", body)
	var env map[string]errorEnvelope
	_ = json.Unmarshal(out, &env)
	if code != http.StatusNotFound || env["error"].Code != tools.CodeNotFound {
		t.Fatalf("status %d body %s, want 404 %s", code, out, tools.CodeNotFound)
	}
}

// TestPostRejectsNonJSONContentType: a cross-origin page can send a
// "simple" POST (text/plain, form types) without a CORS preflight. Requiring
// application/json on every JSON endpoint forces a preflight, which the
// server never grants, so a drive-by page cannot start jobs.
func TestPostRejectsNonJSONContentType(t *testing.T) {
	root := t.TempDir()
	src := writeTinyPNG(t, root)
	ts := newTestServer(t, root)
	body, _ := json.Marshal(convertRequest{Source: fileRef{Path: src}, TargetFormat: "JPEG"})

	for _, ct := range []string{
		"text/plain",
		"text/plain;charset=UTF-8",
		"application/x-www-form-urlencoded",
		"multipart/form-data; boundary=x",
		"",
	} {
		code, out := postAs(t, ts.URL+"/v1/image/convert", ct, body)
		if code != http.StatusBadRequest {
			t.Errorf("Content-Type %q: status %d want 400; body=%s", ct, code, out)
		}
	}
	if n := len(getJobs(t, ts.URL).Jobs); n != 0 {
		t.Fatalf("%d job(s) started by non-JSON requests, want 0", n)
	}
}

// TestPostAcceptsJSONContentTypeWithParams: the check must accept the JSON
// media type with parameters and in any case, as browsers and clients send it.
func TestPostAcceptsJSONContentTypeWithParams(t *testing.T) {
	root := t.TempDir()
	src := writeTinyPNG(t, root)
	ts := newTestServer(t, root)
	body, _ := json.Marshal(convertRequest{Source: fileRef{Path: src}, TargetFormat: "JPEG"})

	for _, ct := range []string{"application/json", "application/json; charset=utf-8", "Application/JSON"} {
		code, out := postAs(t, ts.URL+"/v1/image/convert", ct, body)
		if code != http.StatusAccepted {
			t.Errorf("Content-Type %q: status %d want 202; body=%s", ct, code, out)
			continue
		}
		// Let the job finish before t.TempDir cleanup removes its output dir.
		var jr jobResponse
		if err := json.Unmarshal(out, &jr); err == nil && jr.JobID != "" {
			readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
		}
	}
}
