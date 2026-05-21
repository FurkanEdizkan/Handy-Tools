package pdf

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// PreviewResult is a PNG-encoded render of a PDF's first page.
type PreviewResult struct {
	PNG []byte
}

// Preview renders the first page of a PDF to a PNG thumbnail with pdftoppm.
// It writes nothing the caller has to clean up and returns a MISSING_BINARY
// *tools.Error when pdftoppm is unavailable, so the web gallery can fall back
// to a generic icon.
func Preview(ctx context.Context, source string) (PreviewResult, error) {
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
		return PreviewResult{}, &tools.Error{Code: tools.CodeIO, Message: "temp dir", Detail: err.Error()}
	}
	defer os.RemoveAll(dir)

	// -singlefile renders exactly page 1 to "<stem>.png" (no page suffix).
	stem := filepath.Join(dir, "page")
	cmd := exec.CommandContext(ctx, r.UsedAlias, //nolint:gosec // alias from sysdep, source via CheckPath
		"-png", "-singlefile", "-r", "96", "-f", "1", "-l", "1", source, stem)
	if out, err := cmd.CombinedOutput(); err != nil {
		return PreviewResult{}, &tools.Error{
			Code: tools.CodeIO, Message: "pdftoppm failed", Detail: strings.TrimSpace(string(out)),
		}
	}

	data, err := os.ReadFile(stem + ".png")
	if err != nil {
		return PreviewResult{}, &tools.Error{Code: tools.CodeIO, Message: "read preview", Detail: err.Error()}
	}
	return PreviewResult{PNG: data}, nil
}
