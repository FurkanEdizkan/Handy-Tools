package http

import (
	"net/http"

	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// jobToWire converts a queue.Job snapshot into its JSON wire shape.
func jobToWire(j queue.Job) jobSummary {
	out := jobSummary{
		JobID:       j.ID,
		Tool:        j.Tool,
		Action:      j.Action,
		Status:      jobStatus(j),
		Completed:   j.Completed,
		Fraction:    j.Progress.Fraction,
		CurrentItem: j.Progress.CurrentItem,
		Message:     j.Progress.Message,
		Error:       toolErrorFromProgress(j.Err),
	}
	if !j.StartedAt.IsZero() {
		out.StartedAt = j.StartedAt.UnixMilli()
	}
	return out
}

// jobStatus derives a coarse lifecycle label from a job snapshot. A job with
// no progress emitted yet is "queued" — the queue's emit callback always
// stamps Progress.Tool, so an empty Tool means the Runner hasn't run yet.
func jobStatus(j queue.Job) string {
	switch {
	case j.Err != nil:
		return "failed"
	case j.Completed:
		return "done"
	case j.Progress.Tool == "":
		return "queued"
	default:
		return "running"
	}
}

// handleJobsList returns a snapshot of every job the shared queue knows
// about, oldest first.
func (s *Server) handleJobsList(w http.ResponseWriter, _ *http.Request) {
	jobs := s.Queue.List()
	out := make([]jobSummary, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, jobToWire(j))
	}
	writeJSON(w, http.StatusOK, jobsResponse{Jobs: out})
}

// handleJobsEvents streams every job's lifecycle snapshots (start, each
// progress update, completion) as Server-Sent Events for the lifetime of the
// connection. Unlike GET /v1/jobs/{id}/events it does not replay history —
// callers fetch the initial state via GET /v1/jobs, then subscribe here for
// live updates.
func (s *Server) handleJobsEvents(w http.ResponseWriter, r *http.Request) {
	// Flusher is optional: net/http supplies one and we must call it to push
	// each chunk through the HTTP/1.1 chunked encoder, but Wails' AssetServer
	// ResponseWriter writes straight through a Unix pipe to WebKit — no
	// userspace buffer to flush. Hard-requiring Flusher here was breaking the
	// desktop GUI's Jobs panel entirely (dock counter frozen at 0 because the
	// SSE stream never opened).
	flusher, _ := w.(http.Flusher) //nolint:errcheck // ok=false is handled by flush() being a no-op

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if flusher != nil {
		flusher.Flush()
	}

	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}
	for j := range s.Queue.SubscribeAll(r.Context()) {
		if err := writeSSEJSON(w, flush, jobToWire(j)); err != nil {
			return
		}
	}
}

// handleJobEvents streams a job's progress as Server-Sent Events. It subscribes
// to the shared queue, which replays the job's full history from the start
// before live events, so an SSE reader that connects after the POST returns
// still sees everything.
func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ch, err := s.Queue.Subscribe(r.Context(), id)
	if err != nil {
		writeError(w, &tools.Error{
			Code:    tools.CodeBadRequest,
			Message: "unknown job",
			Detail:  id,
		})
		return
	}

	// Flusher is optional — see handleJobsEvents above for the rationale.
	flusher, _ := w.(http.Flusher) //nolint:errcheck // ok=false is handled by flush() being a no-op

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// X-Accel-Buffering disables proxy buffering for the common nginx case
	// so SSE frames arrive promptly when htoolsd sits behind a reverse proxy.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if flusher != nil {
		flusher.Flush()
	}

	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}
	for p := range ch {
		if err := writeSSEEvent(w, flush, p); err != nil {
			return
		}
	}
}
