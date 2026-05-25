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

// Inspect returns the rename plan without touching the filesystem.
func (h *RenameHandler) Inspect(p RenameParams) ([]rename.Plan, error) {
	srcs, err := h.validateSources(p)
	if err != nil {
		return nil, err
	}
	plans, terr := rename.Inspect(rename.Request{
		Sources:     srcs,
		Pattern:     p.Pattern,
		Replace:     p.Replace,
		OnCollision: parseCollision(p.OnCollision),
	})
	if terr != nil {
		return nil, terr
	}
	return plans, nil
}

// Run executes the rename batch and streams per-file progress.
func (h *RenameHandler) Run(ctx context.Context, p RenameParams, emit func(tools.Progress) error) error {
	srcs, err := h.validateSources(p)
	if err != nil {
		return err
	}
	ch := rename.Run(ctx, rename.Request{
		Sources:     srcs,
		Pattern:     p.Pattern,
		Replace:     p.Replace,
		OnCollision: parseCollision(p.OnCollision),
	})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}
