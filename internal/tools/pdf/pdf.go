// Package pdf wraps PDF operations. Merge, split, and metadata work in pure
// Go via pdfcpu. Rendering pages to images and extracting text are delegated
// to Poppler (pdftoppm / pdftotext) when available.
package pdf

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// Range describes a 1-based inclusive page range. To == 0 means "to the end".
type Range struct {
	From int
	To   int
}

// ToImageRequest renders pages to PNG/JPEG.
type ToImageRequest struct {
	Source     string
	Pages      Range
	DPI        int
	OutputDir  string
	OutputBase string // file stem; default = basename(Source) without ext
	JPEG       bool   // false = PNG
}

// ToImage rasterizes a PDF using pdftoppm.
func ToImage(ctx context.Context, req ToImageRequest) <-chan tools.Progress {
	ch := make(chan tools.Progress, 4)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "pdf"
			p.Action = "to-image"
			p.StartedAt = started
			p.CurrentItem = filepath.Base(req.Source)
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		r := sysdep.Lookup("pdftoppm")
		if !r.Found {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code:    tools.CodeMissingBinary,
				Message: "pdftoppm not found",
				Detail:  "install hint: " + r.Tool.InstallHint["linux"],
			}})
			return
		}

		if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: "create output dir", Detail: err.Error(),
			}})
			return
		}

		base := req.OutputBase
		if base == "" {
			base = strings.TrimSuffix(filepath.Base(req.Source), filepath.Ext(req.Source))
		}
		dpi := req.DPI
		if dpi <= 0 {
			dpi = 150
		}

		args := []string{"-r", strconv.Itoa(dpi)}
		if req.JPEG {
			args = append(args, "-jpeg")
		} else {
			args = append(args, "-png")
		}
		if req.Pages.From > 0 {
			args = append(args, "-f", strconv.Itoa(req.Pages.From))
		}
		if req.Pages.To > 0 {
			args = append(args, "-l", strconv.Itoa(req.Pages.To))
		}
		args = append(args, req.Source, filepath.Join(req.OutputDir, base))

		if err := streamCmd(ctx, r.UsedAlias, args, emit); err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: "pdftoppm failed", Detail: err.Error(),
			}})
			return
		}
		emit(tools.Progress{Completed: true, Fraction: 1, Message: "rendered to " + req.OutputDir})
	}()
	return ch
}

// ToTextRequest extracts plain text.
type ToTextRequest struct {
	Source     string
	Pages      Range
	OutputFile string // "" -> derived next to Source
	Layout     bool
}

// ToText runs pdftotext.
func ToText(ctx context.Context, req ToTextRequest) <-chan tools.Progress {
	ch := make(chan tools.Progress, 4)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "pdf"
			p.Action = "to-text"
			p.StartedAt = started
			p.CurrentItem = filepath.Base(req.Source)
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		r := sysdep.Lookup("pdftotext")
		if !r.Found {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeMissingBinary, Message: "pdftotext not found",
				Detail: "install hint: " + r.Tool.InstallHint["linux"],
			}})
			return
		}

		out := req.OutputFile
		if out == "" {
			base := strings.TrimSuffix(req.Source, filepath.Ext(req.Source))
			out = base + ".txt"
		}

		args := []string{}
		if req.Layout {
			args = append(args, "-layout")
		}
		if req.Pages.From > 0 {
			args = append(args, "-f", strconv.Itoa(req.Pages.From))
		}
		if req.Pages.To > 0 {
			args = append(args, "-l", strconv.Itoa(req.Pages.To))
		}
		args = append(args, req.Source, out)

		if err := streamCmd(ctx, r.UsedAlias, args, emit); err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: "pdftotext failed", Detail: err.Error(),
			}})
			return
		}
		emit(tools.Progress{Completed: true, Fraction: 1, Message: "wrote " + out})
	}()
	return ch
}

// MergeRequest concatenates PDFs in order.
type MergeRequest struct {
	Sources    []string
	OutputFile string
}

// Merge concatenates the given PDFs. The current implementation requires
// pdfcpu CLI as a system dependency; it can be swapped for the library import
// once we add the dependency to go.mod (left as a follow-up to keep this
// commit dep-light).
func Merge(ctx context.Context, req MergeRequest) <-chan tools.Progress {
	ch := make(chan tools.Progress, 4)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "pdf"
			p.Action = "merge"
			p.StartedAt = started
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		if len(req.Sources) < 2 {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeBadRequest, Message: "merge needs at least 2 sources",
			}})
			return
		}

		path, err := exec.LookPath("pdfcpu")
		if err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeMissingBinary, Message: "pdfcpu not found",
				Detail: "install with: go install github.com/pdfcpu/pdfcpu/cmd/pdfcpu@latest",
			}})
			return
		}
		args := append([]string{"merge", req.OutputFile}, req.Sources...)
		if err := streamCmd(ctx, path, args, emit); err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: "pdfcpu merge failed", Detail: err.Error(),
			}})
			return
		}
		emit(tools.Progress{Completed: true, Fraction: 1, Message: "wrote " + req.OutputFile})
	}()
	return ch
}

// SplitRequest describes a split job. Exactly one mode must be set:
//   - PageRanges: each range is collected into its own output PDF.
//   - EveryN: the source is split into consecutive N-page files.
type SplitRequest struct {
	Source     string
	PageRanges []Range // page-ranges mode: one output PDF per range
	EveryN     int     // every-n mode: split into N-page files
	OutputDir  string
}

// Split splits a PDF via the pdfcpu CLI — the same shell-out path Merge uses.
// PageRanges mode runs `pdfcpu collect` once per range; EveryN mode runs
// `pdfcpu split` with a span. pdfcpu is required as a system dependency.
func Split(ctx context.Context, req SplitRequest) <-chan tools.Progress {
	ch := make(chan tools.Progress, 4)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "pdf"
			p.Action = "split"
			p.StartedAt = started
			if p.CurrentItem == "" {
				p.CurrentItem = filepath.Base(req.Source)
			}
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		hasRanges := len(req.PageRanges) > 0
		hasEveryN := req.EveryN > 0
		if hasRanges == hasEveryN {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code:    tools.CodeBadRequest,
				Message: "split needs exactly one mode: page_ranges or every_n",
			}})
			return
		}

		path, err := exec.LookPath("pdfcpu")
		if err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeMissingBinary, Message: "pdfcpu not found",
				Detail: "install with: go install github.com/pdfcpu/pdfcpu/cmd/pdfcpu@latest",
			}})
			return
		}

		if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: "create output dir", Detail: err.Error(),
			}})
			return
		}

		if hasEveryN {
			// pdfcpu split inFile outDir [span]
			args := []string{"split", req.Source, req.OutputDir, strconv.Itoa(req.EveryN)}
			if err := streamCmd(ctx, path, args, emit); err != nil {
				emit(tools.Progress{Completed: true, Err: &tools.Error{
					Code: tools.CodeIO, Message: "pdfcpu split failed", Detail: err.Error(),
				}})
				return
			}
			emit(tools.Progress{Completed: true, Fraction: 1,
				Message: fmt.Sprintf("split every %d page(s) into %s", req.EveryN, req.OutputDir)})
			return
		}

		// page-ranges mode: one `pdfcpu collect` per range.
		stem := strings.TrimSuffix(filepath.Base(req.Source), filepath.Ext(req.Source))
		for i, r := range req.PageRanges {
			if err := ctx.Err(); err != nil {
				emit(tools.Progress{Completed: true, Err: &tools.Error{
					Code: tools.CodeAborted, Message: "split canceled",
				}})
				return
			}
			outFile := filepath.Join(req.OutputDir, fmt.Sprintf("%s_%d.pdf", stem, i+1))
			args := []string{"collect", "-pages", rangeSelector(r), req.Source, outFile}
			if err := streamCmd(ctx, path, args, emit); err != nil {
				emit(tools.Progress{Completed: true, Err: &tools.Error{
					Code: tools.CodeIO, Message: "pdfcpu collect failed", Detail: err.Error(),
				}})
				return
			}
			emit(tools.Progress{
				Level:       tools.SeverityInfo,
				Message:     fmt.Sprintf("[%d/%d] wrote %s", i+1, len(req.PageRanges), outFile),
				Fraction:    float64(i+1) / float64(len(req.PageRanges)),
				CurrentItem: filepath.Base(outFile),
			})
		}
		emit(tools.Progress{Completed: true, Fraction: 1,
			Message: fmt.Sprintf("wrote %d range(s) into %s", len(req.PageRanges), req.OutputDir)})
	}()
	return ch
}

// rangeSelector renders a Range as a pdfcpu page-selection string: "from-to",
// "from-" when To is open-ended, or a single "n".
func rangeSelector(r Range) string {
	if r.To <= 0 {
		return strconv.Itoa(r.From) + "-"
	}
	if r.From == r.To {
		return strconv.Itoa(r.From)
	}
	return strconv.Itoa(r.From) + "-" + strconv.Itoa(r.To)
}

func streamCmd(ctx context.Context, name string, args []string, emit func(tools.Progress)) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	scan := bufio.NewScanner(stdout)
	for scan.Scan() {
		emit(tools.Progress{Level: tools.SeverityInfo, Message: scan.Text()})
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
