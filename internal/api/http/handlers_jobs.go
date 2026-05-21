package http

import (
	"net/http"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

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

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, &tools.Error{
			Code:    tools.CodeIO,
			Message: "streaming not supported by transport",
		})
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// X-Accel-Buffering disables proxy buffering for the common nginx case
	// so SSE frames arrive promptly when htoolsd sits behind a reverse proxy.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	flush := func() { flusher.Flush() }
	for p := range ch {
		if err := writeSSEEvent(w, flush, p); err != nil {
			return
		}
	}
}
