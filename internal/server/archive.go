package server

import (
	"context"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
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

// CompressParams drives ArchiveHandler.Compress. Format may be
// archive.FormatUnknown, in which case archive.Compress infers it from the
// output file extension.
type CompressParams struct {
	Sources          []string
	Format           archive.Format
	Output           string
	Password         string
	CompressionLevel int
}

// Compress packs Sources into a single archive at Output. Every source path
// and the output path are run through CheckPath so the allow-root sandbox is
// enforced for archive creation just as it is for extraction.
func (h *ArchiveHandler) Compress(ctx context.Context, p CompressParams, emit func(tools.Progress) error) error {
	srcs := make([]string, 0, len(p.Sources))
	for _, s := range p.Sources {
		v, err := h.Opts.CheckPath(s)
		if err != nil {
			return err
		}
		srcs = append(srcs, v)
	}
	out, err := h.Opts.CheckPath(p.Output)
	if err != nil {
		return err
	}
	ch := archive.Compress(ctx, archive.CompressRequest{
		Sources:          srcs,
		Format:           p.Format,
		Output:           out,
		Password:         p.Password,
		CompressionLevel: p.CompressionLevel,
	})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}
