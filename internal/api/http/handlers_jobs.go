package http

import (
	"net/http"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j := s.jobs.get(id)
	if j == nil {
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
	_ = j.stream(r.Context(), func(p tools.Progress) error {
		return writeSSEEvent(w, flush, p)
	})
}
