package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools/stripmeta"
)

// cmdStripMeta handles `htools strip-meta <files>... [--in-place]
// [--rollback-on-error]`.
//
// Re-encodes each source in its own format with metadata stripped. The
// stdlib PNG / JPEG encoders we use don't carry EXIF / IPTC / XMP at all,
// so the strip happens implicitly: decode parses the pixels and drops every
// metadata segment, then encode writes only the pixel stream.
//
// Default output is `<base>-stripped<ext>` alongside each source.
// --in-place overwrites the source. --rollback-on-error stops the batch
// on first failure and undoes already-written outputs (or restores
// originals from `.handy-bak` sidecars in --in-place mode).
func cmdStripMeta(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("strip-meta", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	quality := fs.Int("quality", cfg.Image.DefaultJPEGQuality, "JPEG re-encode quality 1..100")
	inPlace := fs.Bool("in-place", false, "overwrite the source file instead of writing <name>-stripped.<ext>")
	rollback := fs.Bool("rollback-on-error", false, "abort on first failure and undo already-written outputs")
	strict := fs.Bool("strict", false, "abort before any work when preflight reports unreadable/missing/unsupported sources")
	dryRun := fs.Bool("dry-run", false, "report preflight issues for the source list and exit; do not strip anything")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, "strip-meta", err.Error())
	}
	if len(srcs) == 0 {
		return usageErr(os.Stderr, "strip-meta", "need at least one source path")
	}

	req := stripmeta.Request{
		Sources:  srcs,
		Quality:  *quality,
		InPlace:  *inPlace,
		Rollback: *rollback,
	}
	opts := progressOpts{quiet: *quiet, json: *asJSON, strict: *strict}

	if *dryRun || *strict {
		ins, terr := stripmeta.Inspect(req)
		if terr != nil {
			fmt.Fprintf(os.Stderr, "strip-meta: %s\n", terr.Error())
			return exitCode(terr)
		}
		if code := runPreflight(ins.Issues, opts); code != 0 {
			return code
		}
		if *dryRun {
			if !*quiet {
				fmt.Fprintf(os.Stderr, "dry-run: %d source(s) ready, %d issue(s)\n", len(srcs)-len(ins.Issues), len(ins.Issues))
			}
			return 0
		}
	}

	return streamProgress(stripmeta.Run(ctx, req), opts)
}
