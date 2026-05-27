package http

import (
	"context"
	"net/http"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// handleRenameInspect is the dry-run preview: returns the rename plan without
// touching the filesystem. The UI calls this before enabling the Run button.
func (s *Server) handleRenameInspect(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	srcs := make([]string, 0, len(req.Sources))
	for _, sref := range req.Sources {
		srcs = append(srcs, sref.Path)
	}
	ins, err := s.Rename.Inspect(server.RenameParams{
		Sources:     srcs,
		Pattern:     req.Pattern,
		Replace:     req.Replace,
		OnCollision: req.OnCollision,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]renamePlan, 0, len(ins.Plans))
	for _, p := range ins.Plans {
		out = append(out, renamePlan{From: p.From, To: p.To})
	}
	issues := make([]pathIssue, 0, len(ins.Issues))
	for _, iss := range ins.Issues {
		issues = append(issues, pathIssue{Path: iss.Path, Code: iss.Code, Detail: iss.Detail})
	}
	writeJSON(w, http.StatusOK, renameInspectResponse{Plans: out, Issues: issues})
}

// handleRenameRun executes the batch and streams one Progress event per file.
func (s *Server) handleRenameRun(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	srcs := make([]string, 0, len(req.Sources))
	for _, sref := range req.Sources {
		srcs = append(srcs, sref.Path)
	}
	params := server.RenameParams{
		Sources:     srcs,
		Pattern:     req.Pattern,
		Replace:     req.Replace,
		OnCollision: req.OnCollision,
	}

	s.enqueue(w, "rename", "run", 30*time.Minute,
		func(ctx context.Context, emit func(tools.Progress)) error {
			return s.Rename.Run(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}
