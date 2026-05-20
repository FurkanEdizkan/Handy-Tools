// Package image performs image format conversion. It uses the Go standard
// library plus golang.org/x/image for PNG, JPEG, GIF, BMP, TIFF, and a
// pure-Go WebP encoder. HEIC decoding is delegated to the system "magick"
// binary when available.
package image

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	"golang.org/x/image/tiff"
	xwebp "golang.org/x/image/webp" // decoder only

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// Format enumerates the formats Handy Tools understands.
type Format int

const (
	FormatUnspecified Format = iota
	FormatPNG
	FormatJPEG
	FormatGIF
	FormatBMP
	FormatTIFF
	FormatWebP
	FormatHEIC
)

func (f Format) Ext() string {
	switch f {
	case FormatPNG:
		return ".png"
	case FormatJPEG:
		return ".jpg"
	case FormatGIF:
		return ".gif"
	case FormatBMP:
		return ".bmp"
	case FormatTIFF:
		return ".tiff"
	case FormatWebP:
		return ".webp"
	case FormatHEIC:
		return ".heic"
	}
	return ""
}

// Options control conversion behavior.
type Options struct {
	Quality       int  // 1..100, used by JPEG (and WebP if a CGO encoder is added later)
	MaxWidth      int  // 0 = keep
	MaxHeight     int  // 0 = keep
	StripMetadata bool // best-effort; encoders here don't carry EXIF anyway
}

// ConvertRequest describes a single conversion.
type ConvertRequest struct {
	Source       string
	TargetFormat Format
	Opts         Options
	Output       string // either a file path or a directory; resolved by Convert
	Overwrite    bool
}

// Convert performs one conversion and streams progress on the returned channel.
// The channel is closed when the conversion completes (successfully or not).
func Convert(ctx context.Context, req ConvertRequest) <-chan tools.Progress {
	ch := make(chan tools.Progress, 4)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "image"
			p.Action = "convert"
			p.StartedAt = started
			p.CurrentItem = filepath.Base(req.Source)
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		emit(tools.Progress{Level: tools.SeverityInfo, Message: "decoding " + filepath.Base(req.Source)})
		img, err := decode(req.Source)
		if err != nil {
			emit(tools.Progress{Completed: true, Level: tools.SeverityError, Err: &tools.Error{
				Code: tools.CodeIO, Message: "decode failed", Detail: err.Error(),
			}})
			return
		}

		if req.Opts.MaxWidth > 0 || req.Opts.MaxHeight > 0 {
			img = resize(img, req.Opts.MaxWidth, req.Opts.MaxHeight)
		}

		outPath, err := resolveOutputPath(req)
		if err != nil {
			emit(tools.Progress{Completed: true, Level: tools.SeverityError, Err: &tools.Error{
				Code: tools.CodeBadRequest, Message: err.Error(),
			}})
			return
		}

		if !req.Overwrite {
			if _, err := os.Stat(outPath); err == nil {
				emit(tools.Progress{Completed: true, Level: tools.SeverityError, Err: &tools.Error{
					Code: tools.CodeBadRequest, Message: "output exists", Detail: outPath,
				}})
				return
			}
		}

		emit(tools.Progress{Level: tools.SeverityInfo, Message: "encoding " + filepath.Base(outPath), Fraction: 0.5})

		if err := encode(outPath, img, req.TargetFormat, req.Opts); err != nil {
			emit(tools.Progress{Completed: true, Level: tools.SeverityError, Err: &tools.Error{
				Code: tools.CodeIO, Message: "encode failed", Detail: err.Error(),
			}})
			return
		}

		emit(tools.Progress{Completed: true, Fraction: 1, Level: tools.SeverityInfo,
			Message: "wrote " + outPath})
	}()
	return ch
}

// BatchConvertRequest describes a multi-file conversion. Every source is
// converted to TargetFormat and written into OutputDir under its own name.
type BatchConvertRequest struct {
	Sources      []string
	TargetFormat Format
	Opts         Options
	OutputDir    string
	Overwrite    bool
}

// BatchConvert converts each source in turn, emitting one tools.Progress per
// file plus a terminal summary. A single file's failure is reported as a
// per-file error event and the batch continues; the terminal event carries an
// error only when every file failed.
func BatchConvert(ctx context.Context, req BatchConvertRequest) <-chan tools.Progress {
	ch := make(chan tools.Progress, 8)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "image"
			p.Action = "batch-convert"
			p.StartedAt = started
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		if len(req.Sources) == 0 {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeBadRequest, Message: "batch convert needs at least one source",
			}})
			return
		}

		total := len(req.Sources)
		failed := 0
		for i, src := range req.Sources {
			if err := ctx.Err(); err != nil {
				emit(tools.Progress{Completed: true, Err: &tools.Error{
					Code: tools.CodeAborted, Message: "batch convert canceled",
				}})
				return
			}
			name := filepath.Base(src)
			outPath, terr := convertOne(ConvertRequest{
				Source:       src,
				TargetFormat: req.TargetFormat,
				Opts:         req.Opts,
				Output:       req.OutputDir,
				Overwrite:    req.Overwrite,
			})
			if terr != nil {
				failed++
				emit(tools.Progress{
					Level:       tools.SeverityError,
					CurrentItem: name,
					Fraction:    float64(i+1) / float64(total),
					Message:     fmt.Sprintf("[%d/%d] %s: %s", i+1, total, name, terr.Message),
				})
				continue
			}
			emit(tools.Progress{
				Level:       tools.SeverityInfo,
				CurrentItem: name,
				Fraction:    float64(i+1) / float64(total),
				Message:     fmt.Sprintf("[%d/%d] wrote %s", i+1, total, filepath.Base(outPath)),
			})
		}

		if failed == total {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: fmt.Sprintf("all %d conversion(s) failed", total),
			}})
			return
		}
		emit(tools.Progress{
			Completed: true, Fraction: 1, Level: tools.SeverityInfo,
			Message: fmt.Sprintf("converted %d/%d image(s)", total-failed, total),
		})
	}()
	return ch
}

// convertOne runs the decode → resize → encode pipeline for one file and
// returns the written path. It does no progress reporting, so both Convert
// and BatchConvert can drive it.
func convertOne(req ConvertRequest) (outPath string, terr *tools.Error) {
	img, err := decode(req.Source)
	if err != nil {
		return "", &tools.Error{Code: tools.CodeIO, Message: "decode failed", Detail: err.Error()}
	}
	if req.Opts.MaxWidth > 0 || req.Opts.MaxHeight > 0 {
		img = resize(img, req.Opts.MaxWidth, req.Opts.MaxHeight)
	}
	outPath, err = resolveOutputPath(req)
	if err != nil {
		return "", &tools.Error{Code: tools.CodeBadRequest, Message: err.Error()}
	}
	if !req.Overwrite {
		if _, statErr := os.Stat(outPath); statErr == nil {
			return "", &tools.Error{Code: tools.CodeBadRequest, Message: "output exists", Detail: outPath}
		}
	}
	if err := encode(outPath, img, req.TargetFormat, req.Opts); err != nil {
		return "", &tools.Error{Code: tools.CodeIO, Message: "encode failed", Detail: err.Error()}
	}
	return outPath, nil
}

func decode(path string) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".heic" || ext == ".heif" {
		return decodeHEIC(path)
	}
	f, err := os.Open(path) //nolint:gosec // user-supplied path is intentional
	if err != nil {
		return nil, err
	}
	defer f.Close()

	switch ext {
	case ".webp":
		return xwebp.Decode(f)
	case ".bmp":
		return bmp.Decode(f)
	case ".tiff", ".tif":
		return tiff.Decode(f)
	}
	img, _, err := image.Decode(f)
	return img, err
}

func decodeHEIC(path string) (image.Image, error) {
	r := sysdep.Lookup("magick")
	if !r.Found {
		return nil, &tools.Error{
			Code:    tools.CodeMissingBinary,
			Message: "HEIC decoding requires ImageMagick",
			Detail:  "install with: " + r.Tool.InstallHint["linux"],
		}
	}
	tmp, err := os.CreateTemp("", "handy-tools-heic-*.png")
	if err != nil {
		return nil, err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	cmd := exec.Command(r.UsedAlias, path, tmp.Name()) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("magick: %s: %w", strings.TrimSpace(string(out)), err)
	}
	f, err := os.Open(tmp.Name())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func encode(path string, img image.Image, f Format, opts Options) error {
	out, err := os.Create(path) //nolint:gosec
	if err != nil {
		return err
	}
	defer out.Close()

	switch f {
	case FormatPNG:
		return png.Encode(out, img)
	case FormatJPEG:
		q := opts.Quality
		if q <= 0 || q > 100 {
			q = 90
		}
		return jpeg.Encode(out, img, &jpeg.Options{Quality: q})
	case FormatGIF:
		return gif.Encode(out, img, nil)
	case FormatBMP:
		return bmp.Encode(out, img)
	case FormatTIFF:
		return tiff.Encode(out, img, nil)
	case FormatWebP:
		return errors.New("webp encoding is not implemented in pure Go yet; convert to PNG/JPEG instead")
	case FormatHEIC:
		return encodeViaMagick(path, img)
	}
	return fmt.Errorf("unsupported target format")
}

// encodeViaMagick writes a PNG to a temp file, then asks magick to convert it
// to the final path. Used for HEIC where there is no pure-Go encoder.
func encodeViaMagick(path string, img image.Image) error {
	r := sysdep.Lookup("magick")
	if !r.Found {
		return &tools.Error{Code: tools.CodeMissingBinary, Message: "HEIC encoding requires ImageMagick"}
	}
	tmp, err := os.CreateTemp("", "handy-tools-encode-*.png")
	if err != nil {
		return err
	}
	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	cmd := exec.Command(r.UsedAlias, tmp.Name(), path) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("magick: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func resize(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxW > 0 && w > maxW {
		h = h * maxW / w
		w = maxW
	}
	if maxH > 0 && h > maxH {
		w = w * maxH / h
		h = maxH
	}
	if w == b.Dx() && h == b.Dy() {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func resolveOutputPath(req ConvertRequest) (string, error) {
	if req.Output == "" {
		base := strings.TrimSuffix(req.Source, filepath.Ext(req.Source))
		return base + req.TargetFormat.Ext(), nil
	}
	info, err := os.Stat(req.Output)
	switch {
	case err == nil && info.IsDir():
		base := strings.TrimSuffix(filepath.Base(req.Source), filepath.Ext(req.Source))
		return filepath.Join(req.Output, base+req.TargetFormat.Ext()), nil
	case errors.Is(err, os.ErrNotExist):
		// Treat as the target file path.
		return req.Output, nil
	case err != nil:
		return "", err
	}
	return req.Output, nil
}

// drainable so tests don't need to import io
var _ io.Reader = (*os.File)(nil)
