package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
)

// cmdConvert handles `htools convert <src>... --format <fmt> ...`.
func cmdConvert(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := fs.String("format", "", "target format: jpeg, png, webp, gif, bmp, tiff, heic (required)")
	quality := fs.Int("quality", cfg.Image.DefaultJPEGQuality, "JPEG quality 1..100")
	maxW := fs.Int("max-width", 0, "downscale width cap (0 = keep)")
	maxH := fs.Int("max-height", 0, "downscale height cap (0 = keep)")
	strip := fs.Bool("strip-metadata", cfg.Image.StripMetadata, "drop EXIF / metadata if the encoder supports it")
	out := fs.String("out", "", "output file (single source) or directory")
	overwrite := fs.Bool("overwrite", false, "replace existing output without renaming")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, "convert", err.Error())
	}
	if len(srcs) == 0 {
		return usageErr(os.Stderr, "convert", "need at least one source path")
	}
	if *format == "" {
		return usageErr(os.Stderr, "convert", "--format is required")
	}
	target, ok := parseImageFormat(*format)
	if !ok {
		return usageErr(os.Stderr, "convert", fmt.Sprintf("unknown --format %q (want jpeg, png, webp, gif, bmp, tiff, heic)", *format))
	}

	opts := image.Options{
		Quality:       *quality,
		MaxWidth:      *maxW,
		MaxHeight:     *maxH,
		StripMetadata: *strip,
	}
	po := progressOpts{quiet: *quiet, json: *asJSON}

	if len(srcs) == 1 {
		ch := image.Convert(ctx, image.ConvertRequest{
			Source:       srcs[0],
			TargetFormat: target,
			Opts:         opts,
			Output:       *out,
			Overwrite:    *overwrite,
		})
		return streamProgress(ch, po)
	}
	// Batch mode: --out must be a directory (defaults to cwd).
	dir := *out
	if dir == "" {
		dir = "."
	}
	ch := image.BatchConvert(ctx, image.BatchConvertRequest{
		Sources:      srcs,
		TargetFormat: target,
		Opts:         opts,
		OutputDir:    dir,
		Overwrite:    *overwrite,
	})
	return streamProgress(ch, po)
}

// parseImageFormat maps a user-typed --format value to image.Format.
func parseImageFormat(s string) (image.Format, bool) {
	switch strings.ToLower(s) {
	case "jpeg", "jpg":
		return image.FormatJPEG, true
	case "png":
		return image.FormatPNG, true
	case "webp":
		return image.FormatWebP, true
	case "gif":
		return image.FormatGIF, true
	case "bmp":
		return image.FormatBMP, true
	case "tiff", "tif":
		return image.FormatTIFF, true
	case "heic", "heif":
		return image.FormatHEIC, true
	}
	return image.FormatUnspecified, false
}
