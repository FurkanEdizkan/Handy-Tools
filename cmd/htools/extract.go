package main

import (
	"context"
	"flag"
	"os"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
)

// cmdExtract handles `htools extract <archive> --out DIR ...`.
func cmdExtract(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", ".", "destination directory")
	password := fs.String("password", "", "optional archive password")
	auto := fs.Bool("auto-multi-part", cfg.Archive.AutoExtractMultiPart, "proceed without confirmation on .partN volumes")
	overwrite := fs.Bool("overwrite", cfg.Archive.OverwriteByDefault, "replace existing files in the destination")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, "extract", err.Error())
	}
	if len(srcs) != 1 {
		return usageErr(os.Stderr, "extract", "need exactly one archive path")
	}

	ch := archive.Extract(ctx, archive.ExtractRequest{
		Source:        srcs[0],
		Destination:   *out,
		Password:      *password,
		Overwrite:     *overwrite,
		AutoMultiPart: *auto,
	})
	return streamProgress(ch, progressOpts{quiet: *quiet, json: *asJSON})
}
