package server

import (
	"context"

	"github.com/furkandedizkan/handy/internal/tools"
	"github.com/furkandedizkan/handy/internal/tools/archive"
)

type ArchiveHandler struct {
	Opts Options
}

type InspectParams struct {
	Source string
}

func (h *ArchiveHandler) Inspect(ctx context.Context, p InspectParams) (*archive.Inspection, error) {
	src, err := h.Opts.CheckPath(p.Source)
	if err != nil {
		return nil, err
	}
	return archive.Inspect(ctx, src)
}

type ExtractParams struct {
	Source        string
	Parts         []string
	Destination   string
	Password      string
	Overwrite     bool
	AutoMultiPart bool
}

func (h *ArchiveHandler) Extract(ctx context.Context, p ExtractParams, emit func(tools.Progress) error) error {
	src, err := h.Opts.CheckPath(p.Source)
	if err != nil {
		return err
	}
	dst, err := h.Opts.CheckPath(p.Destination)
	if err != nil {
		return err
	}
	parts := make([]string, 0, len(p.Parts))
	for _, pp := range p.Parts {
		v, err := h.Opts.CheckPath(pp)
		if err != nil {
			return err
		}
		parts = append(parts, v)
	}
	ch := archive.Extract(ctx, archive.ExtractRequest{
		Source:        src,
		Parts:         parts,
		Destination:   dst,
		Password:      p.Password,
		Overwrite:     p.Overwrite,
		AutoMultiPart: p.AutoMultiPart,
	})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}
