// Package image performs image format conversion. It uses the Go standard
// library plus golang.org/x/image for PNG, JPEG, GIF, BMP, TIFF, and a
// pure-Go WebP encoder. HEIC decoding is delegated to the system "magick"
// binary when available.
package image

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand/v2"
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

// sourceBytes returns the on-disk size of path (0 if unreadable), used only to
// seed the convert ETA estimate.
func sourceBytes(path string) int64 {
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return fi.Size()
	}
	return 0
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

		// Decode + resize + encode are opaque one-shot calls, so animate an ETA
		// bar from the source size while they run. stop() is idempotent; call
		// it before each terminal event so no stray estimate lands after.
		stop := tools.RunEstimator(ctx, emit, tools.Estimator{
			Tool: "image", Action: "convert", InputSize: sourceBytes(req.Source), Start: started,
		}, tools.Progress{Level: tools.SeverityInfo})
		defer stop()

		emit(tools.Progress{Level: tools.SeverityInfo, Message: "decoding " + filepath.Base(req.Source)})
		outPath, terr := convertOne(req, nil, func(outPath string) {
			emit(tools.Progress{Level: tools.SeverityInfo, Message: "encoding " + filepath.Base(outPath)})
		})
		stop()
		if terr != nil {
			emit(tools.Progress{Completed: true, Level: tools.SeverityError, Err: terr})
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
		// BatchConvert creates a missing OutputDir, so probe the nearest
		// existing ancestor instead of flagging the directory as NOT_FOUND.
		dir := req.OutputDir
		for {
			if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
		if issue := tools.CheckOutputDirWritable(dir); issue != nil {
			issues = append(issues, *issue)
		}
	}
	return BatchInspection{Issues: issues}, nil
}

// ensureOutputDir creates a missing batch OutputDir and rejects one that
// exists but is not a directory — otherwise candidateOutputPath would treat
// it as a file name that every output in the batch lands on.
func ensureOutputDir(dir string) *tools.Error {
	info, err := os.Stat(dir)
	switch {
	case err == nil && !info.IsDir():
		return &tools.Error{Code: tools.CodeBadRequest, Message: "output is not a directory", Detail: dir}
	case err == nil:
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return &tools.Error{Code: tools.ClassifyFSError(err), Message: "stat output dir", Detail: err.Error()}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &tools.Error{Code: tools.ClassifyFSError(err), Message: "create output dir", Detail: err.Error()}
	}
	return nil
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

		if req.OutputDir != "" {
			if terr := ensureOutputDir(req.OutputDir); terr != nil {
				emit(tools.Progress{Completed: true, Err: terr})
				return
			}
		}

		total := len(req.Sources)
		workers := req.Parallelism
		if workers <= 0 {
			workers = runtime.GOMAXPROCS(0)
		}
		if workers > total {
			workers = total
		}

		// With Overwrite, two sources that map to the same output (a.png +
		// a.jpg -> a.webp, or d1/a.png + d2/a.png into one OutputDir) must not
		// both write it. The first in source order keeps the natural name;
		// later ones get a -N suffix like the non-overwrite path, skipping
		// every name another source is about to write. Without Overwrite,
		// ReserveUniquePath already hands each worker a distinct name.
		claimed := map[string]struct{}{}
		duplicate := make([]bool, total)
		if req.Overwrite {
			for i, src := range req.Sources {
				c, err := candidateOutputPath(ConvertRequest{Source: src, TargetFormat: req.TargetFormat, Output: req.OutputDir})
				if err != nil {
					continue // surfaces as this file's failure in the worker
				}
				if _, taken := claimed[c]; taken {
					duplicate[i] = true
					continue
				}
				claimed[c] = struct{}{}
			}
		}

		type job struct {
			index int
			src   string
		}
		jobs := make(chan job)
		var (
			wg        sync.WaitGroup
			completed atomic.Int64
			failuresM sync.Mutex
			failures  []tools.Failure
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
					var avoid map[string]struct{}
					if duplicate[j.index] {
						avoid = claimed
					}
					outPath, terr := convertOne(ConvertRequest{
						Source:       j.src,
						TargetFormat: req.TargetFormat,
						Opts:         req.Opts,
						Output:       req.OutputDir,
						Overwrite:    req.Overwrite,
					}, avoid, nil)
					done := completed.Add(1)
					if terr != nil {
						failuresM.Lock()
						failures = append(failures, tools.Failure{Path: j.src, Code: terr.Code, Message: terr.Message})
						failuresM.Unlock()
						emit(tools.Progress{
							Level:       tools.SeverityError,
							CurrentItem: name,
							Fraction:    float64(done) / float64(total),
							Message:     fmt.Sprintf("[%d/%d] %s: %s (%s)", done, total, name, terr.Message, terr.Code),
							Err:         terr,
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
			}, Failures: failures})
			return
		}
		nFailed := len(failures)
		if nFailed == total {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CoalesceFailureCode(failures), Message: fmt.Sprintf("all %d conversion(s) failed", total),
			}, Failures: failures})
			return
		}
		emit(tools.Progress{
			Completed: true, Fraction: 1, Level: tools.SeverityInfo,
			Message:  fmt.Sprintf("converted %d/%d image(s)", total-nFailed, total),
			Failures: failures,
		})
	}()
	return ch
}

// convertOne runs the decode → resize → encode pipeline for one file and
// returns the written path. It does no progress reporting of its own, so both
// Convert and BatchConvert can drive it; onEncode (optional) is told the
// resolved output path just before encoding starts.
//
// avoid is non-nil only for a BatchConvert duplicate under Overwrite: the
// natural output name belongs to another source, so a fresh -N name is
// reserved that is not in avoid (see planOutput).
func convertOne(req ConvertRequest, avoid map[string]struct{}, onEncode func(outPath string)) (string, *tools.Error) {
	src, terr := load(req)
	if terr != nil {
		return "", terr
	}
	if terr := checkEncoder(req.TargetFormat); terr != nil {
		return "", terr
	}
	outPath, reserved, err := planOutput(req, avoid)
	if err != nil {
		code := tools.ClassifyFSError(err)
		if code == tools.CodeIO {
			code = tools.CodeBadRequest
		}
		return "", &tools.Error{Code: code, Message: err.Error()}
	}
	if onEncode != nil {
		onEncode(outPath)
	}
	if err := src.encode(outPath, req.TargetFormat, req.Opts); err != nil {
		if reserved {
			// Release the placeholder so a retry gets the natural name.
			_ = os.Remove(outPath)
		}
		return "", encodeError(err)
	}
	return outPath, nil
}

// decoded is a source ready to encode: a single image, or — for a GIF→GIF
// re-encode of an animation — every frame, so nothing is silently dropped.
type decoded struct {
	img  image.Image
	anim *gif.GIF
}

func (d decoded) encode(path string, f Format, opts Options) error {
	if d.anim != nil {
		return replaceFile(path, f.Ext(), func(out *os.File) error { return gif.EncodeAll(out, d.anim) })
	}
	return encode(path, d.img, f, opts)
}

// load decodes req.Source (and applies any resize). Re-encoding into the
// source's own container format must not lose frames: an animated GIF keeps
// every frame (and resizing one is refused), while a multi-page TIFF or an
// animated PNG is refused outright because their decoders only return the
// first image. Converting to a different format still takes the first frame.
func load(req ConvertRequest) (decoded, *tools.Error) {
	resizing := req.Opts.MaxWidth > 0 || req.Opts.MaxHeight > 0
	var img image.Image
	kind := sniffContainer(req.Source)
	switch {
	case kind == FormatGIF && req.TargetFormat == FormatGIF:
		g, err := decodeGIFAll(req.Source)
		if err != nil {
			return decoded{}, decodeError(err)
		}
		if len(g.Image) > 1 {
			if resizing {
				return decoded{}, &tools.Error{
					Code:    tools.CodeUnsupportedInput,
					Message: "resizing an animated GIF is not supported",
					Detail:  fmt.Sprintf("%s has %d frames", req.Source, len(g.Image)),
				}
			}
			return decoded{anim: g}, nil
		}
		img = g.Image[0]
	case kind == FormatTIFF && req.TargetFormat == FormatTIFF && tiffHasMultiplePages(req.Source):
		return decoded{}, &tools.Error{
			Code:    tools.CodeUnsupportedInput,
			Message: "multi-page TIFF: re-encoding would keep only the first page",
			Detail:  req.Source,
		}
	case kind == FormatPNG && req.TargetFormat == FormatPNG && pngIsAnimated(req.Source):
		return decoded{}, &tools.Error{
			Code:    tools.CodeUnsupportedInput,
			Message: "animated PNG: re-encoding would keep only the first frame",
			Detail:  req.Source,
		}
	}
	if img == nil {
		var err error
		if img, err = decode(req.Source); err != nil {
			return decoded{}, decodeError(err)
		}
	}
	if resizing {
		img = resize(img, req.Opts.MaxWidth, req.Opts.MaxHeight)
	}
	return decoded{img: img}, nil
}

// decodeError normalizes a decode failure the same way encodeError does: a
// structured *tools.Error (MISSING_BINARY from the HEIC magick delegation)
// is preserved; anything else is classified as a filesystem error.
func decodeError(err error) *tools.Error {
	var te *tools.Error
	if errors.As(err, &te) {
		return te
	}
	return &tools.Error{Code: tools.ClassifyFSError(err), Message: "decode failed", Detail: err.Error()}
}

// checkEncoder fails fast — before any output file is reserved or written —
// when the target format can't be encoded: an unknown format, or WebP/HEIC
// without the magick binary.
func checkEncoder(f Format) *tools.Error {
	switch f {
	case FormatPNG, FormatJPEG, FormatGIF, FormatBMP, FormatTIFF:
		return nil
	case FormatWebP, FormatHEIC:
		if r := sysdep.Lookup("magick"); !r.Found {
			return missingMagick(r)
		}
		return nil
	}
	return &tools.Error{Code: tools.CodeBadRequest, Message: "unsupported target format"}
}

func missingMagick(r sysdep.Result) *tools.Error {
	return &tools.Error{
		Code:    tools.CodeMissingBinary,
		Message: "encoding to this format requires ImageMagick",
		Detail:  "install with: " + r.Tool.InstallHint["linux"],
	}
}

// sniffContainer reports the container format from the file's magic bytes
// (GIF, TIFF or PNG), or FormatUnspecified for anything else / unreadable
// files — decode reports those errors properly.
func sniffContainer(path string) Format {
	f, err := os.Open(path) //nolint:gosec // user-supplied path is intentional
	if err != nil {
		return FormatUnspecified
	}
	defer f.Close()
	var magic [8]byte
	n, _ := io.ReadFull(f, magic[:])
	b := magic[:n]
	switch {
	case bytes.HasPrefix(b, []byte("GIF8")):
		return FormatGIF
	case bytes.HasPrefix(b, []byte("II*\x00")), bytes.HasPrefix(b, []byte("MM\x00*")):
		return FormatTIFF
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return FormatPNG
	}
	return FormatUnspecified
}

func decodeGIFAll(path string) (*gif.GIF, error) {
	f, err := os.Open(path) //nolint:gosec // user-supplied path is intentional
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return gif.DecodeAll(f)
}

// tiffHasMultiplePages reports whether the first IFD of a classic TIFF chains
// to another one. Parse problems return false and are left to the decoder.
func tiffHasMultiplePages(path string) bool {
	f, err := os.Open(path) //nolint:gosec // user-supplied path is intentional
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [8]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return false
	}
	var bo binary.ByteOrder
	switch string(hdr[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return false
	}
	if bo.Uint16(hdr[2:4]) != 42 {
		return false
	}
	ifd := int64(bo.Uint32(hdr[4:8]))
	var count [2]byte
	if _, err := f.ReadAt(count[:], ifd); err != nil {
		return false
	}
	var next [4]byte
	if _, err := f.ReadAt(next[:], ifd+2+12*int64(bo.Uint16(count[:]))); err != nil {
		return false
	}
	return bo.Uint32(next[:]) != 0
}

// pngIsAnimated reports whether a PNG carries an APNG acTL chunk before its
// image data. Go's decoder ignores the extra frames, so re-encoding would
// flatten the animation.
func pngIsAnimated(path string) bool {
	f, err := os.Open(path) //nolint:gosec // user-supplied path is intentional
	if err != nil {
		return false
	}
	defer f.Close()
	r := bufio.NewReader(f)
	if _, err := r.Discard(8); err != nil { // signature
		return false
	}
	var hdr [8]byte
	for {
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return false
		}
		switch string(hdr[4:8]) {
		case "acTL":
			return true
		case "IDAT", "IEND":
			return false
		}
		if _, err := r.Discard(int(binary.BigEndian.Uint32(hdr[:4])) + 4); err != nil { // data + crc
			return false
		}
	}
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

// encode writes img to path in format f. The destination is only replaced
// once the encode has fully succeeded (see replaceFile), so a failure never
// destroys what was there — for strip-meta --in-place, the source itself.
func encode(path string, img image.Image, f Format, opts Options) error {
	switch f {
	case FormatWebP, FormatHEIC:
		// No pure-Go WebP/HEIC encoder exists; delegate to the optional
		// magick binary.
		return encodeViaMagick(path, img, f)
	}
	return replaceFile(path, f.Ext(), func(out *os.File) error {
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
		}
		return &tools.Error{Code: tools.CodeBadRequest, Message: "unsupported target format"}
	})
}

// replaceFile writes path through a temp file in the same directory and
// renames it into place only after write succeeds; on failure the temp file
// is removed and path is left untouched. An existing destination keeps its
// permission bits, and a symlinked destination is written through to its
// target, as truncating it in place used to. ext is appended to the temp
// name so tools that infer the format from the extension (magick) see it.
func replaceFile(path, ext string, write func(*os.File) error) (err error) {
	if resolved, rerr := filepath.EvalSymlinks(path); rerr == nil {
		path = resolved
	}
	existing, statErr := os.Stat(path)
	tmp, err := createSibling(path, ext)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if err = write(tmp); err != nil {
		return err
	}
	if statErr == nil {
		if err = tmp.Chmod(existing.Mode().Perm()); err != nil {
			return err
		}
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// createSibling creates a new hidden temp file next to path. Unlike
// os.CreateTemp (always 0600) it uses os.Create's 0666 so the process umask
// decides the final mode of a brand-new output, exactly as before.
func createSibling(path, ext string) (*os.File, error) {
	dir, base := filepath.Split(path)
	for i := 0; i < 100; i++ {
		// The random suffix only needs to be unique, not unpredictable.
		name := filepath.Join(dir, fmt.Sprintf(".%s.handy-tmp-%d%s", base, rand.Uint32(), ext)) //nolint:gosec
		// Sibling of a caller-supplied path, like the output itself.
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666) //nolint:gosec
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return f, err
	}
	return nil, fmt.Errorf("could not create a temp file next to %s", path)
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
// into a sibling temp file carrying the target format's extension (magick
// infers the format from it), which replaceFile renames over path on
// success. Used for HEIC and WebP, which have no pure-Go encoder. The magick
// lookup happens before anything is created.
func encodeViaMagick(path string, img image.Image, f Format) error {
	r := sysdep.Lookup("magick")
	if !r.Found {
		return missingMagick(r)
	}
	tmp, err := os.CreateTemp("", "handy-tools-encode-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	return replaceFile(path, f.Ext(), func(out *os.File) error {
		cmd := exec.Command(r.UsedAlias, tmp.Name(), out.Name()) //nolint:gosec
		if msg, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("magick: %s: %w", strings.TrimSpace(string(msg)), err)
		}
		return nil
	})
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
	// Extreme aspect ratios round the short side down to 0, which no encoder
	// accepts; keep at least one pixel.
	w, h = max(w, 1), max(h, 1)
	if w == b.Dx() && h == b.Dy() {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// planOutput resolves where req's output goes. reserved reports that the
// returned path is a placeholder created here (ReserveUniquePath), which the
// caller removes again if the encode fails.
//
// avoid is set for a BatchConvert duplicate under Overwrite: the natural name
// belongs to another source, so it reserves stem-N.ext instead, skipping the
// names in avoid (the outputs other sources are about to write).
func planOutput(req ConvertRequest, avoid map[string]struct{}) (path string, reserved bool, err error) {
	candidate, err := candidateOutputPath(req)
	if err != nil {
		return "", false, err
	}
	switch {
	case avoid != nil:
		path, err = reserveAvoiding(candidate, avoid)
	case req.Overwrite:
		return candidate, false, nil
	default:
		path, err = tools.ReserveUniquePath(candidate)
	}
	return path, err == nil, err
}

// reserveAvoiding is ReserveUniquePath for a batch duplicate: it starts at
// stem-1.ext and also skips every name in avoid. Like ReserveUniquePath it
// claims the name atomically with O_CREATE|O_EXCL.
func reserveAvoiding(path string, avoid map[string]struct{}) (string, error) {
	stem, ext := tools.SplitOutputExt(path)
	for i := 1; i <= 9999; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, taken := avoid[candidate]; taken {
			continue
		}
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // derived from a caller-supplied path
		if err == nil {
			_ = f.Close()
			return candidate, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find a free output path for %s", path)
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

// drainable so tests don't need to import io
var _ io.Reader = (*os.File)(nil)
