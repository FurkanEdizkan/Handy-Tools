package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
)

// cmdPack handles `htools pack <src>... --format <fmt> --output FILE ...`.
func cmdPack(ctx context.Context, _ config.Config, args []string) int {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := fs.String("format", "", "target format: zip, tar, tar.gz, tar.bz2, tar.zst, tar.xz, 7z (required)")
	output := fs.String("output", "", "output archive path (required)")
	level := fs.Int("compression-level", 0, "0 = format default; 1 fastest .. 9 smallest")
	password := fs.String("password", "", "optional; 7z only")
	overwrite := fs.Bool("overwrite", false, "replace an existing output file; default renames to <stem>-1.<ext>")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, "pack", err.Error())
	}
	if len(srcs) == 0 {
		return usageErr(os.Stderr, "pack", "need at least one source path")
	}
	if *output == "" {
		return usageErr(os.Stderr, "pack", "--output is required")
	}
	if *format == "" {
		return usageErr(os.Stderr, "pack", "--format is required")
	}
	target, ok := parseArchiveFormat(*format)
	if !ok {
		return usageErr(os.Stderr, "pack", fmt.Sprintf("unknown --format %q (want zip, tar, tar.gz, tar.bz2, tar.zst, tar.xz, 7z)", *format))
	}

	ch := archive.Compress(ctx, archive.CompressRequest{
		Sources:          srcs,
		Format:           target,
		Output:           *output,
		Password:         *password,
		CompressionLevel: *level,
		Overwrite:        *overwrite,
	})
	return streamProgress(ch, progressOpts{quiet: *quiet, json: *asJSON})
}

// parseArchiveFormat maps a user-typed --format value to archive.Format.
// Accepts both "tar.gz" and "tgz", "zst" and "tar.zst", etc.
func parseArchiveFormat(s string) (archive.Format, bool) {
	switch strings.ToLower(s) {
	case "zip":
		return archive.FormatZip, true
	case "tar":
		return archive.FormatTar, true
	case "tar.gz", "tgz", "gz":
		return archive.FormatTarGz, true
	case "tar.bz2", "tbz", "tbz2", "bz2":
		return archive.FormatTarBz2, true
	case "tar.zst", "tzst", "zst":
		return archive.FormatTarZst, true
	case "tar.xz", "txz", "xz":
		return archive.FormatTarXz, true
	case "7z":
		return archive.FormatSevenZ, true
	}
	return archive.FormatUnknown, false
}
