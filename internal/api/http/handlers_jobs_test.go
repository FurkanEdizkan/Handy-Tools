package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// getJobs fetches GET /v1/jobs and decodes the envelope.
func getJobs(t *testing.T, ts string) jobsResponse {
	t.Helper()
	resp, err := http.Get(ts + "/v1/jobs") //nolint:noctx // test helper
	if err != nil {
		t.Fatalf("get jobs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/jobs: got %d want 200", resp.StatusCode)
	}
	var jr jobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		t.Fatalf("decode jobs: %v", err)
	}
	return jr
}

// runConvertJob posts an image-convert job and returns its job id, draining
// the per-job SSE stream so the job has reached completion on return.
func runConvertJob(t *testing.T, ts, dir string) string {
	t.Helper()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "out.jpg")
	body, _ := json.Marshal(convertRequest{
		Source:       fileRef{Path: src},
		TargetFormat: "JPEG",
		Options:      imageOptions{Quality: 80},
		Output:       outputRef{File: dst, Overwrite: true},
	})
	resp, err := http.Post(ts+"/v1/image/convert", "application/json", bytes.NewReader(body)) //nolint:noctx // test helper
	if err != nil {
		t.Fatalf("post convert: %v", err)
	}
	var jr jobResponse
	decErr := json.NewDecoder(resp.Body).Decode(&jr)
	resp.Body.Close()
	if decErr != nil {
		t.Fatalf("decode job response: %v", decErr)
	}
	if jr.JobID == "" {
		t.Fatal("empty job_id")
	}
	readSSE(t, ts+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	return jr.JobID
}

func TestJobsListEmpty(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	if jr := getJobs(t, ts.URL); len(jr.Jobs) != 0 {
		t.Fatalf("expected 0 jobs on a fresh server, got %d", len(jr.Jobs))
	}
}

func TestJobsListAfterConvert(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)

	id := runConvertJob(t, ts.URL, dir)

	jr := getJobs(t, ts.URL)
	if len(jr.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d: %+v", len(jr.Jobs), jr.Jobs)
	}
	j := jr.Jobs[0]
	if j.JobID != id {
		t.Fatalf("job id: got %q want %q", j.JobID, id)
	}
	if j.Tool != "image" || j.Action != "convert" {
		t.Fatalf("tool/action: got %q/%q", j.Tool, j.Action)
	}
	if !j.Completed || j.Status != "done" {
		t.Fatalf("status: got completed=%v status=%q, want done", j.Completed, j.Status)
	}
	if j.Error != nil {
		t.Fatalf("unexpected error: %+v", j.Error)
	}
}

func TestJobsEventsStream(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)

	// Open the all-jobs SSE stream first — SubscribeAll only delivers events
	// broadcast after the subscription registers, so the job must be posted
	// after the stream's 200 response confirms the handler is subscribed.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/jobs/events", nil)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sse get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE status: got %d want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("Content-Type: got %q want text/event-stream", got)
	}

	id := runConvertJob(t, ts.URL, dir)

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	sawDone := false
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var js jobSummary
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &js); err != nil {
			t.Fatalf("decode SSE frame: %v: %q", err, line)
		}
		if js.JobID != id {
			continue
		}
		if js.Tool != "image" {
			t.Fatalf("tool: got %q want image", js.Tool)
		}
		if js.Completed && js.Status == "done" {
			sawDone = true
			break
		}
	}
	if !sawDone {
		t.Fatalf("did not observe job %s reaching done on the all-jobs stream", id)
	}
}

// noFlusherWriter is a goroutine-safe ResponseWriter that explicitly does NOT
// implement http.Flusher, mirroring the Wails Linux AssetServer ResponseWriter
// (pkg/assetserver/webview/responsewriter_linux.go) which only satisfies
// http.ResponseWriter. Wails' Write is unbuffered (straight through a Unix
// pipe to WebKit), and we capture into a mutex-guarded buffer here so the
// test goroutine can poll the body concurrently with the handler goroutine
// writing to it.
type noFlusherWriter struct {
	mu      sync.Mutex
	headers http.Header
	body    bytes.Buffer
	code    int
}

func (n *noFlusherWriter) Header() http.Header {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.headers == nil {
		n.headers = http.Header{}
	}
	return n.headers
}

func (n *noFlusherWriter) Write(b []byte) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.code == 0 {
		n.code = http.StatusOK
	}
	return n.body.Write(b)
}

func (n *noFlusherWriter) WriteHeader(code int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.code == 0 {
		n.code = code
	}
}

func (n *noFlusherWriter) Snapshot() (code int, body string, contentType string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	ct := ""
	if n.headers != nil {
		ct = n.headers.Get("Content-Type")
	}
	return n.code, n.body.String(), ct
}

// TestJobsEventsAcceptsWriterWithoutFlusher locks in the fix for the GUI's
// "Jobs queue stays at 0" bug. Wails' Linux AssetServer ResponseWriter does
// not implement http.Flusher, so when the handler hard-required Flusher every
// GUI SSE subscription got 500'd at handshake and the dock never received any
// updates.
//
// The handler is called directly (not via an httptest.NewServer) so we
// bypass the net/http chunked-encoder, which buffers without explicit
// Flush — that buffering is a net/http property, not a Wails one, so
// exercising it would test the wrong thing.
func TestJobsEventsAcceptsWriterWithoutFlusher(t *testing.T) {
	s := New(server.Options{AllowRoots: []string{t.TempDir()}}, queue.New())

	wrapped := &noFlusherWriter{}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/events", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		s.handleJobsEvents(wrapped, req)
		close(done)
	}()

	// SubscribeAll is live-only: it has to register before the Enqueue's
	// broadcast for the snapshot to land in the recorder. Wait until the
	// handler has registered its subscription with the queue.
	deadline := time.Now().Add(2 * time.Second)
	for s.Queue.SubscribeAllCount() == 0 {
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("handler never subscribed to the queue")
		}
		time.Sleep(5 * time.Millisecond)
	}

	id := s.Queue.Enqueue("archive", "extract", func(_ context.Context, _ func(tools.Progress)) {})

	// Wait for the job's terminal snapshot to land in the sink, then tear
	// down. noFlusherWriter is mutex-guarded so concurrent polling here is
	// safe with the handler's writes.
	want := `"job_id":"` + id + `"`
	deadline = time.Now().Add(2 * time.Second)
	for {
		_, body, _ := wrapped.Snapshot()
		if strings.Contains(body, want) {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	code, body, ct := wrapped.Snapshot()
	if code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (handler must accept no-Flusher writers); body=%s", code, body)
	}
	if ct != "text/event-stream" {
		t.Fatalf("content-type: got %q want text/event-stream", ct)
	}
	if !strings.Contains(body, want) {
		t.Fatalf("SSE body missing job %s: %q", id, body)
	}
}
