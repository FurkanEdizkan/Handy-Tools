package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
