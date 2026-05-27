package pdf

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// PreviewPage is a single PDF page rendered to a PNG thumbnail.
type PreviewPage struct {
	Page int    // 1-based page number within the source PDF
	PNG  []byte // PNG-encoded render
}

// PreviewResult holds the rendered pages of a PDF preview, ordered by page.
type PreviewResult struct {
	Pages []PreviewPage
}

// Preview renders a range of PDF pages to PNG thumbnails with pdftoppm.
// An empty range (From and To both 0) renders every page; To == 0 means "to
// the last page". It writes nothing the caller has to clean up and returns a
// MISSING_BINARY *tools.Error when pdftoppm is unavailable, so the web gallery
// can fall back to a generic icon.
func Preview(ctx context.Context, source string, pages Range) (PreviewResult, error) {
	r := sysdep.Lookup("pdftoppm")
	if !r.Found {
		return PreviewResult{}, &tools.Error{
			Code:    tools.CodeMissingBinary,
			Message: "pdftoppm not found",
			Detail:  "install hint: " + r.Tool.InstallHint["linux"],
		}
	}

	dir, err := os.MkdirTemp("", "handy-pdf-preview-*")
	if err != nil {
		return PreviewResult{}, &tools.Error{Code: tools.ClassifyFSError(err), Message: "temp dir", Detail: err.Error()}
	}
	defer os.RemoveAll(dir)

	// pdftoppm writes "<stem>-<page>.png" for each rendered page; the page
	// number is zero-padded to the width of the document's page count.
	stem := filepath.Join(dir, "page")
	args := []string{"-png", "-r", "96"}
	if pages.From > 0 {
		args = append(args, "-f", strconv.Itoa(pages.From))
	}
	if pages.To > 0 {
		args = append(args, "-l", strconv.Itoa(pages.To))
	}
	args = append(args, source, stem)

	cmd := exec.CommandContext(ctx, r.UsedAlias, args...) //nolint:gosec // alias from sysdep, source via CheckPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return PreviewResult{}, &tools.Error{
			Code: tools.CodeIO, Message: "pdftoppm failed", Detail: strings.TrimSpace(string(out)),
		}
	}

	matches, err := filepath.Glob(stem + "-*.png")
	if err != nil {
		return PreviewResult{}, &tools.Error{Code: tools.CodeIO, Message: "scan preview output", Detail: err.Error()}
	}
	if len(matches) == 0 {
		return PreviewResult{}, &tools.Error{
			Code:    tools.CodeIO,
			Message: "pdftoppm produced no pages",
			Detail:  "the requested page range may be outside the document",
		}
	}

	out := make([]PreviewPage, 0, len(matches))
	for _, m := range matches {
		// "<stem>-<page>.png" -> the trailing digits are the page number.
		name := strings.TrimSuffix(filepath.Base(m), ".png")
		page, perr := strconv.Atoi(name[strings.LastIndexByte(name, '-')+1:])
		if perr != nil {
			continue
		}
		data, rerr := os.ReadFile(m) //nolint:gosec // m comes from our own temp-dir glob
		if rerr != nil {
			return PreviewResult{}, &tools.Error{Code: tools.ClassifyFSError(rerr), Message: "read preview", Detail: rerr.Error()}
		}
		out = append(out, PreviewPage{Page: page, PNG: data})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Page < out[j].Page })
	return PreviewResult{Pages: out}, nil
}
