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
	s.runPDFJob(w, "to-image", func(ctx context.Context, id string, j *job) {
		if err := s.PDF.ToImage(ctx, params, func(p tools.Progress) error {
			p.JobID = id
			j.append(p)
			return nil
		}); err != nil {
			j.append(failureProgress(id, "pdf", "to-image", err))
		}
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
	s.runPDFJob(w, "to-text", func(ctx context.Context, id string, j *job) {
		if err := s.PDF.ToText(ctx, params, func(p tools.Progress) error {
			p.JobID = id
			j.append(p)
			return nil
		}); err != nil {
			j.append(failureProgress(id, "pdf", "to-text", err))
		}
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
	s.runPDFJob(w, "merge", func(ctx context.Context, id string, j *job) {
		if err := s.PDF.Merge(ctx, params, func(p tools.Progress) error {
			p.JobID = id
			j.append(p)
			return nil
		}); err != nil {
			j.append(failureProgress(id, "pdf", "merge", err))
		}
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
	s.runPDFJob(w, "split", func(ctx context.Context, id string, j *job) {
		if err := s.PDF.Split(ctx, params, func(p tools.Progress) error {
			p.JobID = id
			j.append(p)
			return nil
		}); err != nil {
			j.append(failureProgress(id, "pdf", "split", err))
		}
	})
}

// runPDFJob is the shared lifecycle for the three async PDF endpoints: it
// creates the job, dispatches the work in a detached goroutine with a long
// timeout, and writes the 202 envelope. The actual work is supplied by the
// caller so each handler keeps its request-specific param parsing.
func (s *Server) runPDFJob(w http.ResponseWriter, _ string, work func(ctx context.Context, id string, j *job)) {
	id, j := s.jobs.create()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()
		work(ctx, id, j)
		j.complete()
	}()
	writeJSON(w, http.StatusAccepted, jobResponse{JobID: id})
}
