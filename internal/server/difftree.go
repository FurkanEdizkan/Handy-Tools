package server

import (
	"context"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/difftree"
)

// DiffTreeHandler adapts the difftree tool package. Mirrors the PDF/Image/
// Archive handler shape so the MCP wrapper can register it the same way.
type DiffTreeHandler struct {
	Opts Options
}

// DiffTreeParams drives DiffTreeHandler.Run. Mode is a string here so the
// transport doesn't have to depend on the difftree package; Run validates
// via difftree.ParseMode.
type DiffTreeParams struct {
	A    string
	B    string
	Mode string
}

// Run validates both roots, then streams one progress per diff entry plus
// the terminal summary from difftree.Run.
func (h *DiffTreeHandler) Run(ctx context.Context, p DiffTreeParams, emit func(tools.Progress) error) error {
	a, err := h.Opts.CheckPath(p.A)
	if err != nil {
		return err
	}
	b, err := h.Opts.CheckPath(p.B)
	if err != nil {
		return err
	}
	mode, _ := difftree.ParseMode(p.Mode)
	ch := difftree.Run(ctx, difftree.Request{A: a, B: b, Mode: mode})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}

// Inspect is the synchronous form — used by transports that want the full
// diff slice up front (e.g., an MCP tool that returns the report as JSON
// instead of replaying a progress stream).
func (h *DiffTreeHandler) Inspect(p DiffTreeParams) ([]difftree.Diff, error) {
	a, err := h.Opts.CheckPath(p.A)
	if err != nil {
		return nil, err
	}
	b, err := h.Opts.CheckPath(p.B)
	if err != nil {
		return nil, err
	}
	mode, _ := difftree.ParseMode(p.Mode)
	diffs, terr := difftree.Inspect(difftree.Request{A: a, B: b, Mode: mode})
	if terr != nil {
		return nil, terr
	}
	return diffs, nil
}
