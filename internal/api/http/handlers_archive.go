package http

import (
	"context"
	"net/http"
	"strings"
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

func (s *Server) handleArchiveCompress(w http.ResponseWriter, r *http.Request) {
	var req compressRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	// An explicit but unrecognised format is a client error (400). An empty
	// format is allowed — archive.Compress infers it from the output path.
	format := archive.FormatUnknown
	if req.Format != "" {
		f, ok := parseArchiveFormat(req.Format)
		if !ok {
			writeError(w, &tools.Error{
				Code:    tools.CodeBadRequest,
				Message: "unknown archive format",
				Detail:  req.Format,
			})
			return
		}
		format = f
	}
	srcs := make([]string, 0, len(req.Sources))
	for _, sref := range req.Sources {
		srcs = append(srcs, sref.Path)
	}
	params := server.CompressParams{
		Sources:          srcs,
		Format:           format,
		Output:           req.Destination.File,
		Password:         req.Password,
		CompressionLevel: req.CompressionLevel,
	}

	// Path-check failures (sources/output outside allow-roots) surface as a
	// terminal SSE error event, consistent with the other async handlers
	// (see handleArchiveExtract / handleImageConvert).
	id, j := s.jobs.create()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()
		if err := s.Archive.Compress(ctx, params, func(p tools.Progress) error {
			p.JobID = id
			j.append(p)
			return nil
		}); err != nil {
			j.append(failureProgress(id, "archive", "compress", err))
		}
		j.complete()
	}()
	writeJSON(w, http.StatusAccepted, jobResponse{JobID: id})
}

// parseArchiveFormat is the inverse of archiveFormatName: it maps the wire
// enum string onto an archive.Format. RAR is intentionally accepted here so
// the request reaches archive.Compress, which rejects it with a clear
// UNSUPPORTED_INPUT error rather than a confusing "unknown format" 400.
func parseArchiveFormat(s string) (archive.Format, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "ZIP":
		return archive.FormatZip, true
	case "TAR":
		return archive.FormatTar, true
	case "TAR_GZ", "TGZ":
		return archive.FormatTarGz, true
	case "TAR_BZ2", "TBZ2":
		return archive.FormatTarBz2, true
	case "TAR_ZST", "TZST":
		return archive.FormatTarZst, true
	case "SEVENZ", "7Z":
		return archive.FormatSevenZ, true
	case "RAR":
		return archive.FormatRar, true
	}
	return archive.FormatUnknown, false
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
