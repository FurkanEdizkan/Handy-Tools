package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
)

// cmdStripMeta handles `htools strip-meta <files>... [--in-place]`.
//
// Re-encodes each source in its own format with image.Options.StripMetadata
// = true. The stdlib PNG / JPEG encoders we use don't carry EXIF / IPTC /
// XMP at all, so the strip happens implicitly: decode parses the pixels and
// drops every metadata segment, then encode writes only the pixel stream.
//
// Default output is `<base>-stripped<ext>` alongside each source. --in-place
// overwrites the source. Convert decodes the file fully into memory before
// opening the output, so overwriting is safe even when source == output.
func cmdStripMeta(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("strip-meta", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	quality := fs.Int("quality", cfg.Image.DefaultJPEGQuality, "JPEG re-encode quality 1..100")
	inPlace := fs.Bool("in-place", false, "overwrite the source file instead of writing <name>-stripped.<ext>")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, "strip-meta", err.Error())
	}
	if len(srcs) == 0 {
		return usageErr(os.Stderr, "strip-meta", "need at least one source path")
	}

	opts := image.Options{
		Quality:       *quality,
		StripMetadata: true,
	}
	po := progressOpts{quiet: *quiet, json: *asJSON}

	var failed int
	for _, src := range srcs {
		if err := ctx.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "strip-meta: aborted: %v\n", err)
			return 2
		}
		target, ok := detectImageFormat(src)
		if !ok {
			failed++
			fmt.Fprintf(os.Stderr, "strip-meta: %s: unknown image extension\n", src)
			continue
		}
		ch := image.Convert(ctx, image.ConvertRequest{
			Source:       src,
			TargetFormat: target,
			Opts:         opts,
			Output:       strippedOutputPath(src, *inPlace),
			Overwrite:    true,
		})
		if code := streamProgress(ch, po); code != 0 {
			failed++
		}
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// detectImageFormat maps a source path's extension to image.Format using the
// same vocabulary as `htools convert --format`. Returns false if the
// extension isn't one of the formats convert knows about (so the caller can
// skip and warn instead of attempting a doomed re-encode).
func detectImageFormat(p string) (image.Format, bool) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(p)), ".")
	return parseImageFormat(ext)
}

// strippedOutputPath computes the default `<base>-stripped<ext>` next to the
// source. With inPlace=true returns the source path verbatim so Convert
// overwrites it.
func strippedOutputPath(src string, inPlace bool) string {
	if inPlace {
		return src
	}
	ext := filepath.Ext(src)
	return strings.TrimSuffix(src, ext) + "-stripped" + ext
}
