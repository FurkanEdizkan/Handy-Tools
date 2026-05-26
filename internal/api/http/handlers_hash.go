package http

import (
	"context"
	"net/http"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func (s *Server) handleHashRun(w http.ResponseWriter, r *http.Request) {
	var req hashRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	srcs := make([]string, 0, len(req.Sources))
	for _, sref := range req.Sources {
		srcs = append(srcs, sref.Path)
	}
	params := server.HashRunParams{Sources: srcs, Algo: req.Algo}

	s.enqueue(w, "hash", "run", 30*time.Minute,
		func(ctx context.Context, emit func(tools.Progress)) error {
			return s.Hash.Run(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

func (s *Server) handleHashVerify(w http.ResponseWriter, r *http.Request) {
	var req hashVerifyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	res, err := s.Hash.Verify(r.Context(), server.HashVerifyParams{
		Manifest: req.Manifest.Path,
		Algo:     req.Algo,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	entries := make([]hashVerifyEntry, 0, len(res.Entries))
	for _, e := range res.Entries {
		entries = append(entries, hashVerifyEntry{
			Path:     e.Path,
			Expected: e.Expected,
			Got:      e.Got,
			OK:       e.OK,
			Err:      e.Err,
		})
	}
	writeJSON(w, http.StatusOK, hashVerifyResponse{
		Entries: entries,
		OK:      res.OK,
		Failed:  res.Failed,
		Missing: res.Missing,
	})
}
