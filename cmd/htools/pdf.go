package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools/pdf"
)

// cmdPDF sub-dispatches `htools pdf merge|split|render|text ...`.
func cmdPDF(ctx context.Context, cfg config.Config, args []string) int {
	if len(args) == 0 {
		return usageErr(os.Stderr, "pdf", "need an operation: merge, split, render, text")
	}
	op, rest := args[0], args[1:]
	switch op {
	case "merge":
		return cmdPDFMerge(ctx, rest)
	case "split":
		return cmdPDFSplit(ctx, rest)
	case "render":
		return cmdPDFRender(ctx, cfg, rest)
	case "text":
		return cmdPDFText(ctx, rest)
	}
	return usageErr(os.Stderr, "pdf", fmt.Sprintf("unknown operation %q (want merge, split, render, text)", op))
}

func cmdPDFMerge(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("pdf merge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "", "output PDF path (required)")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, fs.Name(), err.Error())
	}
	if len(srcs) < 2 {
		return usageErr(os.Stderr, "pdf merge", "need at least two source PDFs")
	}
	if *out == "" {
		return usageErr(os.Stderr, "pdf merge", "--out is required")
	}
	ch := pdf.Merge(ctx, pdf.MergeRequest{Sources: srcs, OutputFile: *out})
	return streamProgress(ch, progressOpts{quiet: *quiet, json: *asJSON})
}

func cmdPDFSplit(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("pdf split", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pages := fs.String("pages", "", "page range FROM-TO (1-based, inclusive); collected into one output PDF")
	every := fs.Int("every", 0, "alternative: split into N-page files")
	out := fs.String("out", "", "output directory (required)")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, fs.Name(), err.Error())
	}
	if len(srcs) != 1 {
		return usageErr(os.Stderr, "pdf split", "need exactly one source PDF")
	}
	if *out == "" {
		return usageErr(os.Stderr, "pdf split", "--out is required")
	}
	if (*pages == "" && *every == 0) || (*pages != "" && *every > 0) {
		return usageErr(os.Stderr, "pdf split", "exactly one of --pages or --every must be set")
	}
	req := pdf.SplitRequest{Source: srcs[0], OutputDir: *out, EveryN: *every}
	if *pages != "" {
		r, err := parseRange(*pages)
		if err != nil {
			return usageErr(os.Stderr, "pdf split", err.Error())
		}
		req.PageRanges = []pdf.Range{r}
	}
	ch := pdf.Split(ctx, req)
	return streamProgress(ch, progressOpts{quiet: *quiet, json: *asJSON})
}

func cmdPDFRender(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("pdf render", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pages := fs.String("pages", "", "page range FROM-TO (optional, default = all)")
	dpi := fs.Int("dpi", cfg.PDF.DefaultDPI, "render resolution")
	asJPEG := fs.Bool("jpeg", false, "write JPEG instead of PNG")
	out := fs.String("out", "", "output directory (required)")
	base := fs.String("base", "", "output filename stem (default: source basename)")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, fs.Name(), err.Error())
	}
	if len(srcs) != 1 {
		return usageErr(os.Stderr, "pdf render", "need exactly one source PDF")
	}
	if *out == "" {
		return usageErr(os.Stderr, "pdf render", "--out is required")
	}
	r := pdf.Range{}
	if *pages != "" {
		got, err := parseRange(*pages)
		if err != nil {
			return usageErr(os.Stderr, "pdf render", err.Error())
		}
		r = got
	}
	ch := pdf.ToImage(ctx, pdf.ToImageRequest{
		Source:     srcs[0],
		Pages:      r,
		DPI:        *dpi,
		OutputDir:  *out,
		OutputBase: *base,
		JPEG:       *asJPEG,
	})
	return streamProgress(ch, progressOpts{quiet: *quiet, json: *asJSON})
}

func cmdPDFText(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("pdf text", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pages := fs.String("pages", "", "page range FROM-TO (optional, default = all)")
	layout := fs.Bool("layout", false, "preserve physical layout (-layout flag of pdftotext)")
	out := fs.String("out", "", "output text file (default: source path with .txt suffix)")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, fs.Name(), err.Error())
	}
	if len(srcs) != 1 {
		return usageErr(os.Stderr, "pdf text", "need exactly one source PDF")
	}
	r := pdf.Range{}
	if *pages != "" {
		got, err := parseRange(*pages)
		if err != nil {
			return usageErr(os.Stderr, "pdf text", err.Error())
		}
		r = got
	}
	ch := pdf.ToText(ctx, pdf.ToTextRequest{
		Source:     srcs[0],
		Pages:      r,
		OutputFile: *out,
		Layout:     *layout,
	})
	return streamProgress(ch, progressOpts{quiet: *quiet, json: *asJSON})
}

// parseRange parses a "FROM-TO" string into a pdf.Range. Either side may be
// omitted (e.g. "3-" means from page 3 to end, "-5" means start to page 5).
// A single number like "4" means just that page.
func parseRange(s string) (pdf.Range, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pdf.Range{}, fmt.Errorf("empty page range")
	}
	if !strings.Contains(s, "-") {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return pdf.Range{}, fmt.Errorf("invalid page %q", s)
		}
		return pdf.Range{From: n, To: n}, nil
	}
	parts := strings.SplitN(s, "-", 2)
	r := pdf.Range{}
	if parts[0] != "" {
		n, err := strconv.Atoi(parts[0])
		if err != nil || n < 1 {
			return pdf.Range{}, fmt.Errorf("invalid FROM in %q", s)
		}
		r.From = n
	}
	if parts[1] != "" {
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 1 {
			return pdf.Range{}, fmt.Errorf("invalid TO in %q", s)
		}
		r.To = n
	}
	if r.From > 0 && r.To > 0 && r.To < r.From {
		return pdf.Range{}, fmt.Errorf("range %q has TO < FROM", s)
	}
	return r, nil
}
