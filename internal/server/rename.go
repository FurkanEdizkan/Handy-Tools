package server

import (
	"context"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/rename"
)

// RenameHandler adapts the rename tool package. Both modes (inspect = dry-run,
// run = execute) are exposed so an MCP tool surface can offer preview-then-
// commit. OnCollision is a string here (parsed via parseCollision) so callers
// don't link in the rename package's enum.
type RenameHandler struct {
	Opts Options
}

// RenameParams drives both Inspect and Run.
type RenameParams struct {
	Sources     []string
	Pattern     string
	Replace     string
	OnCollision string // "error" (default), "skip", "suffix"
}

func parseCollision(s string) rename.Collision {
	switch s {
	case "skip":
		return rename.CollisionSkip
	case "suffix":
		return rename.CollisionSuffix
	}
	return rename.CollisionError
}

// request builds the rename.Request shared by Inspect and Run.
func (h *RenameHandler) request(p RenameParams, srcs []string) rename.Request {
	return rename.Request{
		Sources:     srcs,
		Pattern:     p.Pattern,
		Replace:     p.Replace,
		OnCollision: parseCollision(p.OnCollision),
	}
}

// validateSources runs each source through CheckPath and returns the cleaned
// absolute slice. Shared by Inspect and Run.
func (h *RenameHandler) validateSources(p RenameParams) ([]string, error) {
	srcs := make([]string, 0, len(p.Sources))
	for _, s := range p.Sources {
		v, err := h.Opts.CheckPath(s)
		if err != nil {
			return nil, err
		}
		srcs = append(srcs, v)
	}
	return srcs, nil
}

// checkTargets runs every planned destination through CheckPath. The sources
// are already sandboxed, but the replacement template decides where each file
// lands — "../elsewhere/x" would otherwise move it out of the allow-roots.
// This guard is deliberately independent of whatever the rename package
// validates about the replacement itself.
func (h *RenameHandler) checkTargets(plans []rename.Plan) error {
	for _, pl := range plans {
		if _, err := h.Opts.CheckPath(pl.To); err != nil {
			return err
		}
	}
	return nil
}

// Inspect returns the rename plan plus a preflight list of missing or
// unreadable sources (and unwritable target directories) without touching
// the filesystem. The Issues slice is informational; callers decide whether
// to abort or proceed. A plan whose target escapes the allow-roots is
// rejected outright rather than reported as an Issue.
func (h *RenameHandler) Inspect(p RenameParams) (rename.Inspection, error) {
	srcs, err := h.validateSources(p)
	if err != nil {
		return rename.Inspection{}, err
	}
	ins, terr := rename.Inspect(h.request(p, srcs))
	if terr != nil {
		return rename.Inspection{}, terr
	}
	if err := h.checkTargets(ins.Plans); err != nil {
		return rename.Inspection{}, err
	}
	return ins, nil
}

// Run executes the rename batch and streams per-file progress. The plan is
// computed and target-checked first; nothing is renamed if any target falls
// outside the allow-roots.
func (h *RenameHandler) Run(ctx context.Context, p RenameParams, emit func(tools.Progress) error) error {
	srcs, err := h.validateSources(p)
	if err != nil {
		return err
	}
	req := h.request(p, srcs)
	// A plan error (bad regex, ...) is left for rename.Run to report through
	// its own terminal event, exactly as before.
	if ins, terr := rename.Inspect(req); terr == nil {
		if err := h.checkTargets(ins.Plans); err != nil {
			return err
		}
	}
	ch := rename.Run(ctx, req)
	return forwardProgress(ctx, ch, emit)
}
