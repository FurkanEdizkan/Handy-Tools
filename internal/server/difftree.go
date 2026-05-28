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
//
// Parallelism is the worker-pool size for ModeHash file comparisons in
// difftree.Run; 0 (the default) auto-sizes to runtime.GOMAXPROCS(0).
// Ignored in ModeMTime.
type DiffTreeParams struct {
	A           string
	B           string
	Mode        string
	Parallelism int
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
	ch := difftree.Run(ctx, difftree.Request{A: a, B: b, Mode: mode, Parallelism: p.Parallelism})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}

// FileDiffParams names the two files to diff. Paths are validated through
// the same allow-roots sandbox the tree-level Inspect uses.
type FileDiffParams struct {
	A string
	B string
}

// FileDiff returns a structured unified diff between two files. The wrapper
// adds CheckPath so the GUI's "click a changed row" can't be used to read
// arbitrary files outside the configured allow-roots.
func (h *DiffTreeHandler) FileDiff(p FileDiffParams) (difftree.FileDiffResult, error) {
	a, err := h.Opts.CheckPath(p.A)
	if err != nil {
		return difftree.FileDiffResult{}, err
	}
	b, err := h.Opts.CheckPath(p.B)
	if err != nil {
		return difftree.FileDiffResult{}, err
	}
	res, terr := difftree.FileDiff(a, b)
	if terr != nil {
		return difftree.FileDiffResult{}, terr
	}
	return res, nil
}

// Inspect is the synchronous form — used by transports that want the full
// diff slice up front (e.g., an MCP tool that returns the report as JSON
// instead of replaying a progress stream).
func (h *DiffTreeHandler) Inspect(ctx context.Context, p DiffTreeParams) ([]difftree.Diff, error) {
	a, err := h.Opts.CheckPath(p.A)
	if err != nil {
		return nil, err
	}
	b, err := h.Opts.CheckPath(p.B)
	if err != nil {
		return nil, err
	}
	mode, _ := difftree.ParseMode(p.Mode)
	diffs, terr := difftree.Inspect(ctx, difftree.Request{A: a, B: b, Mode: mode, Parallelism: p.Parallelism})
	if terr != nil {
		return nil, terr
	}
	return diffs, nil
}
