package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/pdf"
)

func (s *Server) handlePDFToImage(w http.ResponseWriter, r *http.Request) {
	var req pdfToImageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	params := server.PDFToImageParams{
		Source:    req.Source.Path,
		From:      req.Pages.From,
		To:        req.Pages.To,
		DPI:       req.DPI,
		OutputDir: req.Output.Directory,
		JPEG:      strings.EqualFold(req.TargetFormat, "JPEG") || strings.EqualFold(req.TargetFormat, "JPG"),
	}
	s.enqueue(w, "pdf", "to-image", 1*time.Hour,
		func(ctx context.Context, emit func(tools.Progress)) error {
			return s.PDF.ToImage(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

func (s *Server) handlePDFToText(w http.ResponseWriter, r *http.Request) {
	var req pdfToTextRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	params := server.PDFToTextParams{
		Source:     req.Source.Path,
		From:       req.Pages.From,
		To:         req.Pages.To,
		Layout:     req.Layout,
		OutputFile: req.Output.File,
	}
	s.enqueue(w, "pdf", "to-text", 1*time.Hour,
		func(ctx context.Context, emit func(tools.Progress)) error {
			return s.PDF.ToText(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

func (s *Server) handlePDFMerge(w http.ResponseWriter, r *http.Request) {
	var req pdfMergeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	srcs := make([]string, 0, len(req.Sources))
	for _, sref := range req.Sources {
		srcs = append(srcs, sref.Path)
	}
	params := server.PDFMergeParams{Sources: srcs, OutputFile: req.Output.File}
	s.enqueue(w, "pdf", "merge", 1*time.Hour,
		func(ctx context.Context, emit func(tools.Progress)) error {
			return s.PDF.Merge(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

func (s *Server) handlePDFSplit(w http.ResponseWriter, r *http.Request) {
	var req pdfSplitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	// page_ranges and every_n are mutually exclusive — exactly one mode.
	hasRanges := len(req.PageRanges) > 0
	if hasRanges == (req.EveryN > 0) {
		writeError(w, &tools.Error{
			Code:    tools.CodeBadRequest,
			Message: "exactly one of page_ranges or every_n must be set",
		})
		return
	}
	ranges := make([]pdf.Range, 0, len(req.PageRanges))
	for _, pr := range req.PageRanges {
		// from is 1-based; to == 0 means "through the last page".
		if pr.From < 1 || (pr.To != 0 && pr.To < pr.From) {
			writeError(w, &tools.Error{
				Code:    tools.CodeBadRequest,
				Message: "invalid page range",
				Detail:  fmt.Sprintf("from=%d to=%d", pr.From, pr.To),
			})
			return
		}
		ranges = append(ranges, pdf.Range{From: pr.From, To: pr.To})
	}
	params := server.PDFSplitParams{
		Source:     req.Source.Path,
		PageRanges: ranges,
		EveryN:     req.EveryN,
		OutputDir:  req.Output.Directory,
	}
	s.enqueue(w, "pdf", "split", 1*time.Hour,
		func(ctx context.Context, emit func(tools.Progress)) error {
			return s.PDF.Split(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}
