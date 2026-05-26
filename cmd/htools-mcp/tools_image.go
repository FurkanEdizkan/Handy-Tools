package main

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
)

// imageConvertInput drives image_convert. Output is either output_file (single
// target file) or output_dir (let the tool pick the name). Setting both is
// rejected by the underlying tool.
type imageConvertInput struct {
	Source        string `json:"source" jsonschema:"absolute path of the source image"`
	TargetFormat  string `json:"target_format" jsonschema:"one of: png, jpeg, gif, bmp, tiff, webp, heic"`
	OutputFile    string `json:"output_file,omitempty" jsonschema:"absolute output file path; mutually exclusive with output_dir"`
	OutputDir     string `json:"output_dir,omitempty" jsonschema:"absolute output directory; the tool picks the basename"`
	Quality       int    `json:"quality,omitempty" jsonschema:"JPEG quality 1..100; ignored for lossless formats"`
	MaxWidth      int    `json:"max_width,omitempty" jsonschema:"resize so width <= max_width, preserving aspect ratio (0 = keep)"`
	MaxHeight     int    `json:"max_height,omitempty" jsonschema:"resize so height <= max_height, preserving aspect ratio (0 = keep)"`
	Overwrite     bool   `json:"overwrite,omitempty" jsonschema:"replace an existing output instead of disambiguating with a numeric suffix"`
	StripMetadata bool   `json:"strip_metadata,omitempty" jsonschema:"best-effort: drop EXIF/IPTC/XMP on encode"`
}

type imageBatchConvertInput struct {
	Sources       []string `json:"sources" jsonschema:"absolute paths of source images"`
	TargetFormat  string   `json:"target_format" jsonschema:"one of: png, jpeg, gif, bmp, tiff, webp, heic"`
	OutputDir     string   `json:"output_dir" jsonschema:"absolute directory all converted files are written into"`
	Quality       int      `json:"quality,omitempty" jsonschema:"JPEG quality 1..100"`
	MaxWidth      int      `json:"max_width,omitempty" jsonschema:"resize cap, 0 = keep"`
	MaxHeight     int      `json:"max_height,omitempty" jsonschema:"resize cap, 0 = keep"`
	Overwrite     bool     `json:"overwrite,omitempty" jsonschema:"replace existing outputs instead of suffixing"`
	StripMetadata bool     `json:"strip_metadata,omitempty" jsonschema:"drop metadata on re-encode"`
	Parallelism   int      `json:"parallelism,omitempty" jsonschema:"worker pool size; 0 (default) auto-sizes to host GOMAXPROCS"`
}

type imageStripMetaInput struct {
	Sources   []string `json:"sources" jsonschema:"absolute paths of source images (kept in their original format)"`
	OutputDir string   `json:"output_dir" jsonschema:"absolute directory the cleaned copies are written into"`
	Quality   int      `json:"quality,omitempty" jsonschema:"JPEG quality 1..100 for the re-encode"`
	Overwrite bool     `json:"overwrite,omitempty" jsonschema:"replace an existing output instead of suffixing"`
}

// parseImageFormat normalises the string from the MCP input into the
// image.Format enum used by the tool package. Returns FormatUnspecified for
// unknown strings; the tool will then surface a BAD_REQUEST.
func parseImageFormat(s string) image.Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "png":
		return image.FormatPNG
	case "jpeg", "jpg":
		return image.FormatJPEG
	case "gif":
		return image.FormatGIF
	case "bmp":
		return image.FormatBMP
	case "tiff", "tif":
		return image.FormatTIFF
	case "webp":
		return image.FormatWebP
	case "heic", "heif":
		return image.FormatHEIC
	}
	return image.FormatUnspecified
}

// formatExt picks the destination extension for image_strip_meta when the
// caller doesn't supply one. We re-encode in the same format the source is
// in, so the basename keeps its extension from the source; OutputDir takes
// care of writing into a fresh directory.
func formatExt(srcExt string) image.Format {
	switch strings.ToLower(srcExt) {
	case ".png":
		return image.FormatPNG
	case ".jpg", ".jpeg":
		return image.FormatJPEG
	case ".gif":
		return image.FormatGIF
	case ".bmp":
		return image.FormatBMP
	case ".tif", ".tiff":
		return image.FormatTIFF
	case ".webp":
		return image.FormatWebP
	case ".heic", ".heif":
		return image.FormatHEIC
	}
	return image.FormatUnspecified
}

func registerImageTools(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "image_convert",
		Description: "Convert a single image between formats (png, jpeg, gif, bmp, tiff, webp, heic). WebP and HEIC encoding require the magick binary.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in imageConvertInput) (*mcp.CallToolResult, any, error) {
		res := drainProgress(ctx, "image", "convert", func(emit func(tools.Progress) error) error {
			return h.Image.Convert(ctx, server.ConvertParams{
				Source:       in.Source,
				TargetFormat: parseImageFormat(in.TargetFormat),
				Quality:      in.Quality,
				MaxWidth:     in.MaxWidth,
				MaxHeight:    in.MaxHeight,
				OutputDir:    in.OutputDir,
				OutputFile:   in.OutputFile,
				Overwrite:    in.Overwrite,
			}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "image_batch_convert",
		Description: "Convert many images to the same format in one job; one progress event per file plus a terminal summary.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in imageBatchConvertInput) (*mcp.CallToolResult, any, error) {
		res := drainProgress(ctx, "image", "batch_convert", func(emit func(tools.Progress) error) error {
			return h.Image.BatchConvert(ctx, server.BatchConvertParams{
				Sources:      in.Sources,
				TargetFormat: parseImageFormat(in.TargetFormat),
				Quality:      in.Quality,
				MaxWidth:     in.MaxWidth,
				MaxHeight:    in.MaxHeight,
				OutputDir:    in.OutputDir,
				Overwrite:    in.Overwrite,
				Parallelism:  in.Parallelism,
			}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "image_strip_meta",
		Description: "Re-encode images into output_dir with EXIF/IPTC/XMP stripped; keeps each source in its original format.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in imageStripMetaInput) (*mcp.CallToolResult, any, error) {
		// The image package re-encodes via the requested TargetFormat — for
		// a strip-meta batch we want to keep each source in its own format.
		// To preserve the per-format behaviour without duplicating the batch
		// runner, group sources by detected source extension and submit one
		// batch per group. For mixed batches this means several drains; for
		// the common single-format case it's just one.
		groups := map[image.Format][]string{}
		for _, s := range in.Sources {
			ext := ""
			if i := strings.LastIndex(s, "."); i >= 0 {
				ext = s[i:]
			}
			f := formatExt(ext)
			groups[f] = append(groups[f], s)
		}
		// Single combined result; messages accumulate across batches.
		combined := drainProgress(ctx, "image", "strip_meta", func(emit func(tools.Progress) error) error {
			for f, srcs := range groups {
				if f == image.FormatUnspecified {
					// Surface as an error progress event so the user sees which paths were skipped.
					_ = emit(tools.Progress{
						Tool: "image", Action: "strip_meta",
						Level: tools.SeverityError,
						Err: &tools.Error{
							Code: tools.CodeUnsupportedInput, Message: "unrecognised image extension",
							Detail: strings.Join(srcs, ", "),
						},
					})
					continue
				}
				if err := h.Image.BatchConvert(ctx, server.BatchConvertParams{
					Sources:      srcs,
					TargetFormat: f,
					Quality:      in.Quality,
					MaxWidth:     0,
					MaxHeight:    0,
					OutputDir:    in.OutputDir,
					Overwrite:    in.Overwrite,
				}, emit); err != nil {
					return err
				}
			}
			return nil
		})
		return combined, nil, nil
	})
}
