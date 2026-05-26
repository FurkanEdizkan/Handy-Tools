package server

import (
	"context"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/hash"
)

// HashHandler adapts the hash tool package to the same wire-agnostic shape as
// the PDF/Image/Archive handlers. New transports (gRPC, HTTP, MCP) wrap these
// methods rather than calling internal/tools/hash directly so the allow-root
// sandbox is enforced uniformly.
type HashHandler struct {
	Opts Options
}

// HashRunParams drives HashHandler.Run. Algo is a string here (rather than
// hash.Algo) so transports can pass the user-typed value through without
// linking into the tool package; Run validates via hash.ParseAlgo.
type HashRunParams struct {
	Sources []string
	Algo    string
}

// Run hashes every source after path-validating it and forwards progress to
// emit. Unknown Algo strings surface a BAD_REQUEST via the tool package's own
// terminal progress event — no special handling needed here.
func (h *HashHandler) Run(ctx context.Context, p HashRunParams, emit func(tools.Progress) error) error {
	srcs := make([]string, 0, len(p.Sources))
	for _, s := range p.Sources {
		v, err := h.Opts.CheckPath(s)
		if err != nil {
			return err
		}
		srcs = append(srcs, v)
	}
	algo, _ := hash.ParseAlgo(p.Algo)
	ch := hash.Run(ctx, hash.Request{Sources: srcs, Algo: algo})
	for prog := range ch {
		if err := emit(prog); err != nil {
			return err
		}
	}
	return nil
}

// HashVerifyParams drives HashHandler.Verify.
type HashVerifyParams struct {
	Manifest string
	Algo     string
}

// HashVerifyResult is the per-entry verification report Verify produces.
// Surfaced as-is to transports so callers can render the table they want.
type HashVerifyResult struct {
	Entries []hash.VerifyEntry
	OK      int
	Failed  int
	Missing int
}

// Verify reads the manifest at Manifest (path-validated), recomputes each
// entry's digest, and returns the per-row report. Unlike the other handlers,
// Verify is synchronous — the underlying hash.Verify returns the full slice
// in one shot. emit is left out of the signature because there is no streamed
// progress to forward.
func (h *HashHandler) Verify(_ context.Context, p HashVerifyParams) (*HashVerifyResult, error) {
	manifest, err := h.Opts.CheckPath(p.Manifest)
	if err != nil {
		return nil, err
	}
	algo, _ := hash.ParseAlgo(p.Algo)
	entries, terr := hash.Verify(manifest, algo)
	if terr != nil {
		return nil, terr
	}
	out := &HashVerifyResult{Entries: entries}
	for _, e := range entries {
		switch {
		case e.OK:
			out.OK++
		case e.Err != "":
			out.Missing++
		default:
			out.Failed++
		}
	}
	return out, nil
}
