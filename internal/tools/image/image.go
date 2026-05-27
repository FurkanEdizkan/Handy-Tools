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
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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
				Code: tools.ClassifyFSError(err), Message: "decode failed", Detail: err.Error(),
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

		emit(tools.Progress{Level: tools.SeverityInfo, Message: "encoding " + filepath.Base(outPath), Fraction: 0.5})

		if err := encode(outPath, img, req.TargetFormat, req.Opts); err != nil {
			emit(tools.Progress{Completed: true, Level: tools.SeverityError, Err: encodeError(err)})
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

	// Parallelism caps the number of concurrent worker goroutines.
	// 0 (the default) auto-sizes to runtime.GOMAXPROCS(0). 1 forces
	// serial — useful in tests that depend on event ordering.
	Parallelism int
}

// BatchInspection is the result of InspectBatch.
type BatchInspection struct {
	Issues []tools.PathIssue
}

// InspectBatch is the dry-run / preflight for BatchConvert: it stats every
// source and probes the output directory for writability, returning Issues
// without touching any image data. Use this from `htools convert --dry-run`
// and the `image_batch_inspect` MCP tool to surface "this file disappeared"
// or "you can't write here" before a long batch starts.
func InspectBatch(req BatchConvertRequest) (BatchInspection, *tools.Error) {
	if len(req.Sources) == 0 {
		return BatchInspection{}, &tools.Error{Code: tools.CodeBadRequest, Message: "batch needs at least one source"}
	}
	issues := tools.StatInputs(req.Sources)
	if req.OutputDir != "" {
		if issue := tools.CheckOutputDirWritable(req.OutputDir); issue != nil {
			issues = append(issues, *issue)
		}
	}
	return BatchInspection{Issues: issues}, nil
}

// BatchConvert converts every source concurrently and emits one
// tools.Progress per file plus a terminal summary. A single file's failure
// is reported as a per-file error event and the batch continues; the
// terminal event carries an error only when every file failed.
//
// Per-file events arrive in worker-completion order, not source-list order,
// when Parallelism != 1. Callers that need ordered output (the CLI's
// non-JSON --quiet=false mode) print them as they arrive — the order is
// stable within a single run because the workload is deterministic, but
// don't assert exact ordering in tests.
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
		workers := req.Parallelism
		if workers <= 0 {
			workers = runtime.GOMAXPROCS(0)
		}
		if workers > total {
			workers = total
		}

		type job struct {
			index int
			src   string
		}
		jobs := make(chan job)
		var (
			wg        sync.WaitGroup
			completed atomic.Int64
			failed    atomic.Int64
		)

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					if err := ctx.Err(); err != nil {
						return
					}
					name := filepath.Base(j.src)
					outPath, terr := convertOne(ConvertRequest{
						Source:       j.src,
						TargetFormat: req.TargetFormat,
						Opts:         req.Opts,
						Output:       req.OutputDir,
						Overwrite:    req.Overwrite,
					})
					done := completed.Add(1)
					if terr != nil {
						failed.Add(1)
						emit(tools.Progress{
							Level:       tools.SeverityError,
							CurrentItem: name,
							Fraction:    float64(done) / float64(total),
							Message:     fmt.Sprintf("[%d/%d] %s: %s", done, total, name, terr.Message),
						})
						continue
					}
					emit(tools.Progress{
						Level:       tools.SeverityInfo,
						CurrentItem: name,
						Fraction:    float64(done) / float64(total),
						Message:     fmt.Sprintf("[%d/%d] wrote %s", done, total, filepath.Base(outPath)),
					})
				}
			}()
		}

		// Feed sources; bail early on context cancellation.
	feed:
		for i, src := range req.Sources {
			select {
			case <-ctx.Done():
				break feed
			case jobs <- job{index: i, src: src}:
			}
		}
		close(jobs)
		wg.Wait()

		if err := ctx.Err(); err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeAborted, Message: "batch convert canceled",
			}})
			return
		}
		nFailed := int(failed.Load())
		if nFailed == total {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: fmt.Sprintf("all %d conversion(s) failed", total),
			}})
			return
		}
		emit(tools.Progress{
			Completed: true, Fraction: 1, Level: tools.SeverityInfo,
			Message: fmt.Sprintf("converted %d/%d image(s)", total-nFailed, total),
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
		return "", &tools.Error{Code: tools.ClassifyFSError(err), Message: "decode failed", Detail: err.Error()}
	}
	if req.Opts.MaxWidth > 0 || req.Opts.MaxHeight > 0 {
		img = resize(img, req.Opts.MaxWidth, req.Opts.MaxHeight)
	}
	outPath, err = resolveOutputPath(req)
	if err != nil {
		return "", &tools.Error{Code: tools.CodeBadRequest, Message: err.Error()}
	}
	if err := encode(outPath, img, req.TargetFormat, req.Opts); err != nil {
		return "", encodeError(err)
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
		// No pure-Go WebP encoder exists; delegate to the optional magick
		// binary — the same path HEIC uses.
		return encodeViaMagick(path, img)
	case FormatHEIC:
		return encodeViaMagick(path, img)
	}
	return fmt.Errorf("unsupported target format")
}

// encodeError normalizes an encode failure: a structured *tools.Error (e.g.
// MISSING_BINARY from the magick delegation) is preserved as-is so its code
// and install hint reach the caller; anything else collapses to IO_ERROR.
func encodeError(err error) *tools.Error {
	var te *tools.Error
	if errors.As(err, &te) {
		return te
	}
	return &tools.Error{Code: tools.ClassifyFSError(err), Message: "encode failed", Detail: err.Error()}
}

// encodeViaMagick writes a PNG to a temp file, then asks magick to convert it
// to the final path — magick infers the target format from the path's
// extension. Used for HEIC and WebP, which have no pure-Go encoder.
func encodeViaMagick(path string, img image.Image) error {
	r := sysdep.Lookup("magick")
	if !r.Found {
		return &tools.Error{
			Code:    tools.CodeMissingBinary,
			Message: "encoding to this format requires ImageMagick",
			Detail:  "install with: " + r.Tool.InstallHint["linux"],
		}
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
	candidate, err := candidateOutputPath(req)
	if err != nil {
		return "", err
	}
	if req.Overwrite {
		return candidate, nil
	}
	return disambiguatePath(candidate)
}

// candidateOutputPath computes the natural output path before collision
// disambiguation: source's base name with the target extension, dropped into
// req.Output if that's a directory, or used verbatim if Output is an explicit
// file path.
func candidateOutputPath(req ConvertRequest) (string, error) {
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

// disambiguatePath returns a path that is guaranteed not to collide with an
// existing file at the moment this function returns. It tries the requested
// path first, then path-1, path-2, ... up to a 9999 cap. The check uses
// os.OpenFile with O_CREATE|O_EXCL so the result is atomic against other
// concurrent callers — a stat-then-create loop would race under parallelism
// (two workers can both see "free" and both write). The reserved file is
// closed immediately after creation; encode() reopens it with O_TRUNC.
//
// On overflow (every candidate up to the cap is taken) returns an error so
// the caller surfaces a clean failure instead of silently overwriting.
func disambiguatePath(path string) (string, error) {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	const maxAttempts = 9999
	for i := 0; i <= maxAttempts; i++ {
		candidate := path
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d%s", base, i, ext)
		}
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec
		if err == nil {
			f.Close()
			return candidate, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find a free output path: %s, %s-1..%s-%d all exist",
		path, base, base, maxAttempts)
}

// drainable so tests don't need to import io
var _ io.Reader = (*os.File)(nil)
