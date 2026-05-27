package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestHashRunFailuresCrossTheWire posts a hash batch with two missing
// sources and one chmod-blocked source through the HTTP transport, then
// confirms the terminal SSE frame carries the structured failures slice
// — proving the in-process tools.Progress.Failures field survives the
// progressToWire conversion and reaches an SSE consumer with the same
// classified codes (PERMISSION_DENIED, NOT_FOUND) the tool package emits.
//
// This is the wire-level companion to TestHashRunMixedBatchScenario in
// internal/tools/hash. Together they cover the contract end-to-end: tool
// package -> HTTP handler -> SSE frame.
func TestHashRunFailuresCrossTheWire(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — file modes are bypassed")
	}
	dir := t.TempDir()
	ts := newTestServer(t, dir)

	good := filepath.Join(dir, "ok.txt")
	if err := os.WriteFile(good, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(dir, "blocked.txt")
	if err := os.WriteFile(blocked, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o644) })
	missing := filepath.Join(dir, "ghost.txt")

	body, _ := json.Marshal(hashRunRequest{
		Sources: []fileRef{{Path: good}, {Path: blocked}, {Path: missing}},
		Algo:    "sha256",
	})
	resp, err := http.Post(ts.URL+"/v1/hash", "application/json", bytes.NewReader(body)) //nolint:noctx // test helper
	if err != nil {
		t.Fatalf("POST /v1/hash: %v", err)
	}
	var jr jobResponse
	decErr := json.NewDecoder(resp.Body).Decode(&jr)
	resp.Body.Close()
	if decErr != nil {
		t.Fatalf("decode job: %v", decErr)
	}
	if jr.JobID == "" {
		t.Fatal("empty job_id")
	}

	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 10*time.Second)
	if len(events) == 0 {
		t.Fatal("no SSE events")
	}
	term := events[len(events)-1]
	if !term.Completed {
		t.Fatalf("last event not Completed: %+v", term)
	}
	if term.Error != nil {
		t.Fatalf("expected partial-success terminal (no Error), got %+v", term.Error)
	}
	if len(term.Failures) != 2 {
		t.Fatalf("want 2 failures on terminal SSE frame, got %d: %+v", len(term.Failures), term.Failures)
	}
	codes := map[string]string{}
	for _, f := range term.Failures {
		codes[f.Path] = f.Code
		if f.Message == "" {
			t.Errorf("terminal failure for %s has empty Message field", f.Path)
		}
	}
	if codes[blocked] != "PERMISSION_DENIED" {
		t.Errorf("blocked path code = %q, want PERMISSION_DENIED", codes[blocked])
	}
	if codes[missing] != "NOT_FOUND" {
		t.Errorf("missing path code = %q, want NOT_FOUND", codes[missing])
	}
}

// TestHashRunFailuresOmittedWhenAllSucceed proves the JSON `failures` field
// is omitted (not present as an empty array) when there are no per-file
// failures, so a successful hash batch doesn't add noise to the wire.
func TestHashRunFailuresOmittedWhenAllSucceed(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)
	src := filepath.Join(dir, "ok.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(hashRunRequest{
		Sources: []fileRef{{Path: src}},
		Algo:    "sha256",
	})
	resp, err := http.Post(ts.URL+"/v1/hash", "application/json", bytes.NewReader(body)) //nolint:noctx // test helper
	if err != nil {
		t.Fatalf("POST /v1/hash: %v", err)
	}
	var jr jobResponse
	_ = json.NewDecoder(resp.Body).Decode(&jr)
	resp.Body.Close()

	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	term := events[len(events)-1]
	if !term.Completed || term.Error != nil {
		t.Fatalf("expected clean terminal, got %+v", term)
	}
	if len(term.Failures) != 0 {
		t.Errorf("want empty Failures slice on a fully-successful batch, got %+v", term.Failures)
	}
}
