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
	diffs, err := s.DiffTree.Inspect(r.Context(), server.DiffTreeParams{
		A:           req.A.Path,
		B:           req.B.Path,
		Mode:        req.Mode,
		Parallelism: req.Parallelism,
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

// handleDiffTreeFile returns a unified diff between two files. The GUI
// expands a "changed" row to call this; the response is structured so the
// client can render coloured rows without re-parsing the raw diff text.
func (s *Server) handleDiffTreeFile(w http.ResponseWriter, r *http.Request) {
	var req diffTreeFileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	res, err := s.DiffTree.FileDiff(server.FileDiffParams{A: req.A.Path, B: req.B.Path})
	if err != nil {
		writeError(w, err)
		return
	}
	lines := make([]diffTreeFileLine, 0, len(res.Lines))
	for _, l := range res.Lines {
		lines = append(lines, diffTreeFileLine{
			Kind: string(l.Kind),
			Text: l.Text,
			AOld: l.AOld,
			BNew: l.BNew,
		})
	}
	writeJSON(w, http.StatusOK, diffTreeFileResponse{
		Binary:    res.Binary,
		Truncated: res.Truncated,
		Identical: res.Identical,
		Lines:     lines,
	})
}
