package server

import (
	"context"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/pdf"
)

type PDFHandler struct {
	Opts Options
}

type PDFToImageParams struct {
	Source    string
	From, To  int
	DPI       int
	OutputDir string
	JPEG      bool
}

func (h *PDFHandler) ToImage(ctx context.Context, p PDFToImageParams, emit func(tools.Progress) error) error {
	src, err := h.Opts.CheckPath(p.Source)
	if err != nil {
		return err
	}
	out, err := h.Opts.CheckPath(p.OutputDir)
	if err != nil {
		return err
	}
	ch := pdf.ToImage(ctx, pdf.ToImageRequest{
		Source:    src,
		Pages:     pdf.Range{From: p.From, To: p.To},
		DPI:       p.DPI,
		OutputDir: out,
		JPEG:      p.JPEG,
	})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}

type PDFToTextParams struct {
	Source     string
	From, To   int
	Layout     bool
	OutputFile string
}

func (h *PDFHandler) ToText(ctx context.Context, p PDFToTextParams, emit func(tools.Progress) error) error {
	src, err := h.Opts.CheckPath(p.Source)
	if err != nil {
		return err
	}
	out := ""
	if p.OutputFile != "" {
		v, err := h.Opts.CheckPath(p.OutputFile)
		if err != nil {
			return err
		}
		out = v
	}
	ch := pdf.ToText(ctx, pdf.ToTextRequest{
		Source:     src,
		Pages:      pdf.Range{From: p.From, To: p.To},
		Layout:     p.Layout,
		OutputFile: out,
	})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}

type PDFMergeParams struct {
	Sources    []string
	OutputFile string
}

func (h *PDFHandler) Merge(ctx context.Context, p PDFMergeParams, emit func(tools.Progress) error) error {
	srcs := make([]string, 0, len(p.Sources))
	for _, s := range p.Sources {
		v, err := h.Opts.CheckPath(s)
		if err != nil {
			return err
		}
		srcs = append(srcs, v)
	}
	out, err := h.Opts.CheckPath(p.OutputFile)
	if err != nil {
		return err
	}
	ch := pdf.Merge(ctx, pdf.MergeRequest{Sources: srcs, OutputFile: out})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}

// PDFSplitParams drives PDFHandler.Split. PageRanges and EveryN are the two
// mutually-exclusive split modes; the caller (transport handler) is expected
// to have validated that exactly one is set.
type PDFSplitParams struct {
	Source     string
	PageRanges []pdf.Range
	EveryN     int
	OutputDir  string
}

func (h *PDFHandler) Split(ctx context.Context, p PDFSplitParams, emit func(tools.Progress) error) error {
	src, err := h.Opts.CheckPath(p.Source)
	if err != nil {
		return err
	}
	out, err := h.Opts.CheckPath(p.OutputDir)
	if err != nil {
		return err
	}
	ch := pdf.Split(ctx, pdf.SplitRequest{
		Source:     src,
		PageRanges: p.PageRanges,
		EveryN:     p.EveryN,
		OutputDir:  out,
	})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}
