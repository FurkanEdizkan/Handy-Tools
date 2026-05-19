package http

import (
	"context"
	"net/http"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
)

func (s *Server) handleArchiveInspect(w http.ResponseWriter, r *http.Request) {
	var req inspectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	ins, err := s.Archive.Inspect(r.Context(), server.InspectParams{Source: req.Source.Path})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inspectResponse{
		Format:                archiveFormatName(ins.Format),
		MultiPart:             ins.MultiPart,
		DetectedParts:         ins.DetectedParts,
		MissingParts:          ins.MissingParts,
		UncompressedSizeBytes: ins.UncompressedSz,
		EntryCount:            ins.EntryCount,
		RequiresPassword:      ins.RequiresPwd,
		RequiresBinary:        ins.RequiresBinary,
		BinaryAvailable:       ins.BinaryAvailable,
	})
}

func (s *Server) handleArchiveExtract(w http.ResponseWriter, r *http.Request) {
	var req extractRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	parts := make([]string, 0, len(req.Parts))
	for _, p := range req.Parts {
		parts = append(parts, p.Path)
	}
	params := server.ExtractParams{
		Source:        req.Source.Path,
		Parts:         parts,
		Destination:   req.DestinationDir,
		Password:      req.Password,
		Overwrite:     req.Overwrite,
		AutoMultiPart: req.AutoAcceptMultiPart,
	}

	id, j := s.jobs.create()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()
		if err := s.Archive.Extract(ctx, params, func(p tools.Progress) error {
			p.JobID = id
			j.append(p)
			return nil
		}); err != nil {
			j.append(failureProgress(id, "archive", "extract", err))
		}
		j.complete()
	}()
	writeJSON(w, http.StatusAccepted, jobResponse{JobID: id})
}

func archiveFormatName(f archive.Format) string {
	switch f {
	case archive.FormatZip:
		return "ZIP"
	case archive.FormatTar:
		return "TAR"
	case archive.FormatTarGz:
		return "TAR_GZ"
	case archive.FormatTarBz2:
		return "TAR_BZ2"
	case archive.FormatTarZst:
		return "TAR_ZST"
	case archive.FormatRar:
		return "RAR"
	case archive.FormatSevenZ:
		return "SEVENZ"
	}
	return "UNSPECIFIED"
}
