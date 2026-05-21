package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/buildinfo"
	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// writeTinyPNG mirrors internal/tools/image/image_test.go's helper so the HTTP
// tests stay self-contained — they exercise the full transport without needing
// a fixture file checked into testdata/.
func writeTinyPNG(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "in.png")
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 64), G: uint8(y * 64), B: 128, A: 255})
		}
	}
	f, err := os.Create(src)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return src
}

// newTestServer builds an HTTP server scoped to dir as the only allow-root.
// The returned *httptest.Server uses its own goroutine and is cleaned up by
// the test's t.Cleanup hook.
func newTestServer(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	s := New(server.Options{AllowRoots: []string{dir}}, queue.New())
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestSysdepEndpoint(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp, err := http.Get(ts.URL + "/v1/sysdep")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	var out []sysdepResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected at least one known sysdep tool")
	}
	// Every entry should at least carry a Name; Found may be true or false
	// depending on what's installed on the test host.
	for _, r := range out {
		if r.Name == "" {
			t.Fatalf("sysdep result missing name: %+v", r)
		}
	}
}

func TestImageConvertRejectsUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)
	body, _ := json.Marshal(convertRequest{
		Source:       fileRef{Path: filepath.Join(dir, "in.png")},
		TargetFormat: "BOGUS",
		Output:       outputRef{Directory: dir},
	})
	resp, err := http.Post(ts.URL+"/v1/image/convert", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
	var env struct {
		Error errorEnvelope `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Code != tools.CodeBadRequest {
		t.Fatalf("code: got %q want %q", env.Error.Code, tools.CodeBadRequest)
	}
}

func TestImageConvertEndToEnd(t *testing.T) {
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "out.jpg")

	ts := newTestServer(t, dir)

	body, _ := json.Marshal(convertRequest{
		Source:       fileRef{Path: src},
		TargetFormat: "JPEG",
		Options:      imageOptions{Quality: 80},
		Output:       outputRef{File: dst, Overwrite: true},
	})

	resp, err := http.Post(ts.URL+"/v1/image/convert", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d want 202; body=%s", resp.StatusCode, b)
	}
	var jr jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if jr.JobID == "" {
		t.Fatal("empty job_id")
	}

	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)

	var lastCompleted bool
	var sawError *errorEnvelope
	for _, e := range events {
		if e.Error != nil {
			sawError = e.Error
		}
		if e.Completed {
			lastCompleted = true
		}
	}
	if sawError != nil {
		t.Fatalf("unexpected SSE error: %+v", sawError)
	}
	if !lastCompleted {
		t.Fatalf("did not observe Completed: events=%+v", events)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("output file missing: %v", err)
	}
}

func TestImageConvertOutsideAllowRoots(t *testing.T) {
	dir := t.TempDir()
	// allow-root is a sibling dir, so the source path won't be permitted.
	allow := t.TempDir()
	src := writeTinyPNG(t, dir)

	s := New(server.Options{AllowRoots: []string{allow}}, queue.New())
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	body, _ := json.Marshal(convertRequest{
		Source:       fileRef{Path: src},
		TargetFormat: "JPEG",
		Output:       outputRef{File: filepath.Join(dir, "out.jpg"), Overwrite: true},
	})

	resp, err := http.Post(ts.URL+"/v1/image/convert", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	// Path-check failure happens inside the work goroutine; the POST
	// still returns 202 + a job_id. The SSE stream surfaces the failure.
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status: got %d want 202", resp.StatusCode)
	}
	var jr jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		t.Fatalf("decode: %v", err)
	}

	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	var failure *errorEnvelope
	for _, e := range events {
		if e.Error != nil {
			failure = e.Error
		}
	}
	if failure == nil {
		t.Fatalf("expected terminal error event, got %+v", events)
	}
	if !strings.Contains(strings.ToLower(failure.Message), "allowed root") {
		t.Fatalf("unexpected failure message: %q", failure.Message)
	}
}

func TestJobEventsUnknownID(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp, err := http.Get(ts.URL + "/v1/jobs/does-not-exist/events")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

// readSSE consumes an SSE stream until either the stream closes or a Completed
// event is observed (whichever comes first), respecting timeout.
func readSSE(t *testing.T, url string, timeout time.Duration) []progressEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("SSE status: got %d want 200; body=%s", resp.StatusCode, b)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("Content-Type: got %q want text/event-stream", got)
	}

	var events []progressEvent
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var e progressEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e); err != nil {
			t.Fatalf("decode SSE: %v: %q", err, line)
		}
		events = append(events, e)
		if e.Completed {
			return events
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		t.Fatalf("scan: %v", err)
	}
	return events
}

// TestEndpointMethodMismatch documents that the Go 1.22 ServeMux refuses to
// dispatch GET requests to POST-only handlers — guards against accidental
// route registration regressions.
func TestEndpointMethodMismatch(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp, err := http.Get(ts.URL + "/v1/image/convert")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d want 405", resp.StatusCode)
	}
}

// writeTextFile drops a small file under dir and returns its path.
func writeTextFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// writeNamedPNG writes a tiny PNG at dir/name and returns its path — the
// batch-convert tests need several distinct source images.
func writeNamedPNG(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 64), G: uint8(y * 64), B: 128, A: 255})
		}
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", name, err)
	}
	return p
}

func TestArchiveCompressEndToEnd(t *testing.T) {
	dir := t.TempDir()
	srcA := writeTextFile(t, dir, "a.txt", "alpha")
	srcB := writeTextFile(t, dir, "b.txt", "bravo")
	out := filepath.Join(dir, "out.zip")

	ts := newTestServer(t, dir)
	body, _ := json.Marshal(compressRequest{
		Sources:     []fileRef{{Path: srcA}, {Path: srcB}},
		Destination: outputRef{File: out},
		Format:      "ZIP",
	})
	resp, err := http.Post(ts.URL+"/v1/archive/compress", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d want 202; body=%s", resp.StatusCode, raw)
	}
	var jr jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if jr.JobID == "" {
		t.Fatal("empty job_id")
	}

	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	var done bool
	var sawError *errorEnvelope
	for _, e := range events {
		if e.Error != nil {
			sawError = e.Error
		}
		if e.Completed {
			done = true
		}
	}
	if sawError != nil {
		t.Fatalf("unexpected SSE error: %+v", sawError)
	}
	if !done {
		t.Fatalf("did not observe Completed: events=%+v", events)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
}

func TestArchiveCompressOutsideAllowRoots(t *testing.T) {
	dir := t.TempDir()
	allow := t.TempDir() // a sibling dir — the source below won't be permitted
	src := writeTextFile(t, dir, "a.txt", "x")

	s := New(server.Options{AllowRoots: []string{allow}}, queue.New())
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	body, _ := json.Marshal(compressRequest{
		Sources:     []fileRef{{Path: src}},
		Destination: outputRef{File: filepath.Join(allow, "out.zip")},
		Format:      "ZIP",
	})
	resp, err := http.Post(ts.URL+"/v1/archive/compress", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	// Path-check failure happens inside the work goroutine; the POST still
	// returns 202 and the SSE stream surfaces the failure (same convention
	// as TestImageConvertOutsideAllowRoots).
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status: got %d want 202", resp.StatusCode)
	}
	var jr jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	var failure *errorEnvelope
	for _, e := range events {
		if e.Error != nil {
			failure = e.Error
		}
	}
	if failure == nil {
		t.Fatalf("expected terminal error event, got %+v", events)
	}
	if !strings.Contains(strings.ToLower(failure.Message), "allowed root") {
		t.Fatalf("unexpected failure message: %q", failure.Message)
	}
}

func TestArchiveCompressRejectsUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)
	body, _ := json.Marshal(compressRequest{
		Sources:     []fileRef{{Path: filepath.Join(dir, "a.txt")}},
		Destination: outputRef{File: filepath.Join(dir, "out.zip")},
		Format:      "BOGUS",
	})
	resp, err := http.Post(ts.URL+"/v1/archive/compress", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
	var env struct {
		Error errorEnvelope `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Code != tools.CodeBadRequest {
		t.Fatalf("code: got %q want %q", env.Error.Code, tools.CodeBadRequest)
	}
}

// postPDFSplit POSTs body to /v1/pdf/split and returns the response.
func postPDFSplit(t *testing.T, ts *httptest.Server, req pdfSplitRequest) *http.Response {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(ts.URL+"/v1/pdf/split", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}

// assertSplitAccepted checks the 202 + job_id envelope and that the job's SSE
// stream reaches a terminal Completed event. It does NOT assert success: the
// CI runner has no pdfcpu binary, so pdf.Split terminates with a MISSING_BINARY
// error there — either way the transport must deliver a Completed event.
// The caller owns resp.Body — it must defer-close it (keeps bodyclose happy).
func assertSplitAccepted(t *testing.T, ts *httptest.Server, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d want 202; body=%s", resp.StatusCode, raw)
	}
	var jr jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if jr.JobID == "" {
		t.Fatal("empty job_id")
	}
	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	done := false
	for _, e := range events {
		if e.Completed {
			done = true
		}
	}
	if !done {
		t.Fatalf("did not observe a terminal Completed event: %+v", events)
	}
}

func TestPDFSplitPageRangesAccepted(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)
	src := writeTextFile(t, dir, "in.pdf", "%PDF-1.4 placeholder")
	resp := postPDFSplit(t, ts, pdfSplitRequest{
		Source:     fileRef{Path: src},
		PageRanges: []pageRange{{From: 1, To: 3}, {From: 5, To: 7}},
		Output:     outputRef{Directory: dir},
	})
	defer resp.Body.Close()
	assertSplitAccepted(t, ts, resp)
}

func TestPDFSplitEveryNAccepted(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)
	src := writeTextFile(t, dir, "in.pdf", "%PDF-1.4 placeholder")
	resp := postPDFSplit(t, ts, pdfSplitRequest{
		Source: fileRef{Path: src},
		EveryN: 2,
		Output: outputRef{Directory: dir},
	})
	defer resp.Body.Close()
	assertSplitAccepted(t, ts, resp)
}

func TestPDFSplitRejectsInvalidRange(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)
	resp := postPDFSplit(t, ts, pdfSplitRequest{
		Source:     fileRef{Path: filepath.Join(dir, "in.pdf")},
		PageRanges: []pageRange{{From: 5, To: 2}}, // to < from
		Output:     outputRef{Directory: dir},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

func TestPDFSplitRejectsAmbiguousMode(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)
	// Neither page_ranges nor every_n set — the mode is ambiguous.
	resp := postPDFSplit(t, ts, pdfSplitRequest{
		Source: fileRef{Path: filepath.Join(dir, "in.pdf")},
		Output: outputRef{Directory: dir},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

func TestImageBatchConvertEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeNamedPNG(t, dir, "a.png")
	writeNamedPNG(t, dir, "b.png")

	ts := newTestServer(t, dir)
	body, _ := json.Marshal(batchConvertRequest{
		Sources:      []fileRef{{Path: filepath.Join(dir, "a.png")}, {Path: filepath.Join(dir, "b.png")}},
		TargetFormat: "JPEG",
		Options:      imageOptions{Quality: 80},
		Output:       outputRef{Directory: dir, Overwrite: true},
	})
	resp, err := http.Post(ts.URL+"/v1/image/batch-convert", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d want 202; body=%s", resp.StatusCode, raw)
	}
	var jr jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		t.Fatalf("decode: %v", err)
	}

	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	perFile := map[string]bool{}
	var done bool
	var sawError *errorEnvelope
	for _, e := range events {
		if e.CurrentItem != "" {
			perFile[e.CurrentItem] = true
		}
		if e.Error != nil {
			sawError = e.Error
		}
		if e.Completed {
			done = true
		}
	}
	if sawError != nil {
		t.Fatalf("unexpected SSE error: %+v", sawError)
	}
	if !done {
		t.Fatalf("did not observe Completed: events=%+v", events)
	}
	if !perFile["a.png"] || !perFile["b.png"] {
		t.Fatalf("missing per-file progress events: %v", perFile)
	}
	for _, name := range []string{"a.jpg", "b.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("output %s missing: %v", name, err)
		}
	}
}

// TestImageBatchConvertContinuesOnPartialFailure pins #17's behaviour: one
// bad source does not abort the batch — it lands as a per-file ERROR event
// and the terminal event still completes without an error envelope.
func TestImageBatchConvertContinuesOnPartialFailure(t *testing.T) {
	dir := t.TempDir()
	writeNamedPNG(t, dir, "good.png")

	ts := newTestServer(t, dir)
	body, _ := json.Marshal(batchConvertRequest{
		Sources: []fileRef{
			{Path: filepath.Join(dir, "good.png")},
			{Path: filepath.Join(dir, "missing.png")}, // never created
		},
		TargetFormat: "JPEG",
		Output:       outputRef{Directory: dir, Overwrite: true},
	})
	resp, err := http.Post(ts.URL+"/v1/image/batch-convert", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status: got %d want 202", resp.StatusCode)
	}
	var jr jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		t.Fatalf("decode: %v", err)
	}

	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	var sawErrorLevel, terminalClean bool
	for _, e := range events {
		if e.Level == "ERROR" {
			sawErrorLevel = true
		}
		if e.Completed {
			terminalClean = e.Error == nil
		}
	}
	if !sawErrorLevel {
		t.Fatalf("expected a per-file ERROR event for the missing source: %+v", events)
	}
	if !terminalClean {
		t.Fatalf("terminal event should complete without an error (1/2 succeeded): %+v", events)
	}
	if _, err := os.Stat(filepath.Join(dir, "good.jpg")); err != nil {
		t.Fatalf("good.jpg missing — batch did not continue past the failure: %v", err)
	}
}

func TestImageBatchConvertRejectsUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, dir)
	body, _ := json.Marshal(batchConvertRequest{
		Sources:      []fileRef{{Path: filepath.Join(dir, "a.png")}},
		TargetFormat: "BOGUS",
		Output:       outputRef{Directory: dir},
	})
	resp, err := http.Post(ts.URL+"/v1/image/batch-convert", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

func TestHealthEndpoint(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	// Sleep past one whole second so the truncated uptime is > 0.
	time.Sleep(1100 * time.Millisecond)

	resp, err := http.Get(ts.URL + "/v1/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	var hr healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if hr.Version != buildinfo.Version {
		t.Errorf("version: got %q want %q", hr.Version, buildinfo.Version)
	}
	if hr.UptimeSeconds < 1 {
		t.Errorf("uptime_seconds: got %d, want > 0", hr.UptimeSeconds)
	}
	if len(hr.Transports) == 0 {
		t.Errorf("transports empty; want grpc + http")
	}
}
