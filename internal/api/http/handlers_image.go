package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
)

func (s *Server) handleImageConvert(w http.ResponseWriter, r *http.Request) {
	var req convertRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	fmt, ok := parseImageFormat(req.TargetFormat)
	if !ok {
		writeError(w, &tools.Error{
			Code:    tools.CodeBadRequest,
			Message: "unknown target_format",
			Detail:  req.TargetFormat,
		})
		return
	}
	params := server.ConvertParams{
		Source:       req.Source.Path,
		TargetFormat: fmt,
		Quality:      req.Options.Quality,
		MaxWidth:     req.Options.MaxWidth,
		MaxHeight:    req.Options.MaxHeight,
		OutputDir:    req.Output.Directory,
		OutputFile:   req.Output.File,
		Overwrite:    req.Output.Overwrite,
	}

	s.enqueue(w, "image", "convert", 30*time.Minute,
		func(ctx context.Context, emit func(tools.Progress)) error {
			return s.Image.Convert(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

func (s *Server) handleImageBatchConvert(w http.ResponseWriter, r *http.Request) {
	var req batchConvertRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	fmt, ok := parseImageFormat(req.TargetFormat)
	if !ok {
		writeError(w, &tools.Error{
			Code:    tools.CodeBadRequest,
			Message: "unknown target_format",
			Detail:  req.TargetFormat,
		})
		return
	}
	srcs := make([]string, 0, len(req.Sources))
	for _, sref := range req.Sources {
		srcs = append(srcs, sref.Path)
	}
	params := server.BatchConvertParams{
		Sources:      srcs,
		TargetFormat: fmt,
		Quality:      req.Options.Quality,
		MaxWidth:     req.Options.MaxWidth,
		MaxHeight:    req.Options.MaxHeight,
		OutputDir:    req.Output.Directory,
		Overwrite:    req.Output.Overwrite,
		Parallelism:  req.Parallelism,
	}

	// One job for the whole batch; image.BatchConvert emits one Progress per
	// file (file name in CurrentItem) plus a terminal summary. A single
	// file's failure is reported per-file and the batch continues — see #17.
	s.enqueue(w, "image", "batch-convert", 1*time.Hour,
		func(ctx context.Context, emit func(tools.Progress)) error {
			return s.Image.BatchConvert(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

// parseImageFormat maps the wire enum string to the image.Format value the
// tool layer expects. Empty / unknown values yield (FormatUnspecified, false)
// so callers can return a clear 400.
func parseImageFormat(s string) (image.Format, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "PNG":
		return image.FormatPNG, true
	case "JPEG", "JPG":
		return image.FormatJPEG, true
	case "GIF":
		return image.FormatGIF, true
	case "BMP":
		return image.FormatBMP, true
	case "TIFF", "TIF":
		return image.FormatTIFF, true
	case "WEBP":
		return image.FormatWebP, true
	case "HEIC", "HEIF":
		return image.FormatHEIC, true
	}
	return image.FormatUnspecified, false
}

// failureProgress wraps a non-tool error into a terminal Progress so SSE
// subscribers receive a structured failure instead of a silent close. The
// queue's emit stamps JobID, so it is not set here.
func failureProgress(tool, action string, err error) tools.Progress {
	te, ok := err.(*tools.Error)
	if !ok {
		te = &tools.Error{Code: tools.CodeIO, Message: err.Error()}
	}
	return tools.Progress{
		Tool:      tool,
		Action:    action,
		Level:     tools.SeverityError,
		Completed: true,
		Err:       te,
	}
}
