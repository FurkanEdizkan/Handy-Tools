package http

import (
	"net/http"

	"github.com/furkandedizkan/handy-tools/internal/server"
)

// handleDiffTreeInspect runs the diff synchronously and returns the full
// report. The synchronous shape mirrors archive/inspect — the UI wants the
// table up front, not a streamed series of one-entry events.
func (s *Server) handleDiffTreeInspect(w http.ResponseWriter, r *http.Request) {
	var req diffTreeInspectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	diffs, err := s.DiffTree.Inspect(server.DiffTreeParams{
		A:    req.A.Path,
		B:    req.B.Path,
		Mode: req.Mode,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	entries := make([]diffEntry, 0, len(diffs))
	for _, d := range diffs {
		entries = append(entries, diffEntry{
			Path:   d.Path,
			Status: string(d.Status),
			Reason: d.Reason,
		})
	}
	writeJSON(w, http.StatusOK, diffTreeInspectResponse{Entries: entries})
}
