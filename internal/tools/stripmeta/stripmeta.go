// Package stripmeta re-encodes images to drop EXIF / IPTC / XMP metadata.
// The pure-Go encoders (PNG, JPEG, BMP, GIF, TIFF) don't carry metadata at
// all, so decode-then-encode implicitly strips it. This package is the
// batch driver: one Run kicks off a per-file loop, emits one Progress event
// per file, and (when Rollback is set) undoes anything it managed to write
// if any file fails.
package stripmeta

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
)

// Request describes a strip-metadata batch.
type Request struct {
	// Sources is the list of files to re-encode. Each must be a recognised
	// image extension (png, jpg, jpeg, bmp, gif, tiff). Unrecognised
	// extensions are reported as a Failure entry and the batch continues
	// (or aborts when Rollback is set).
	Sources []string

	// Quality is the JPEG re-encode quality 1..100. Ignored for formats
	// without a quality knob.
	Quality int

	// InPlace overwrites the source file. When false (default), each output
	// is written next to the source as `<base>-stripped<ext>`. Rollback in
	// InPlace mode uses a `<source>.handy-bak` sidecar — see the package doc
	// for the hard-kill caveat.
	InPlace bool

	// Rollback enables best-effort undo on the first per-file failure.
	// !InPlace: every successfully written `-stripped` file is deleted.
	// InPlace: each successful overwrite restores the original from its
	// `.handy-bak` sidecar.
	Rollback bool
}

// Inspection is the result of Inspect: preflight Issues for sources that
// don't exist, aren't readable, or aren't a recognised image format.
type Inspection struct {
	Issues []tools.PathIssue
}

// Inspect is the dry-run / preflight for Run. Stats every source and flags
// any with an unrecognised extension as UNSUPPORTED_INPUT.
func Inspect(req Request) (Inspection, *tools.Error) {
	if len(req.Sources) == 0 {
		return Inspection{}, &tools.Error{Code: tools.CodeBadRequest, Message: "strip-meta needs at least one source"}
	}
	issues := tools.StatInputs(req.Sources)
	known := make(map[string]struct{}, len(issues))
	for _, iss := range issues {
		known[iss.Path] = struct{}{}
	}
	for _, src := range req.Sources {
		if _, alreadyReported := known[src]; alreadyReported {
			continue
		}
		if _, ok := detectImageFormat(src); !ok {
			issues = append(issues, tools.PathIssue{
				Path:   src,
				Code:   tools.CodeUnsupportedInput,
				Detail: "unrecognised image extension",
			})
		}
	}
	return Inspection{Issues: issues}, nil
}

// Run re-encodes every source and streams one Progress event per file plus
// a terminal summary. The terminal event's Failures slice carries one entry
// per file that couldn't be processed (skipped, decode/encode failures, or
// rollback failures).
//
// With Rollback=true the first failure stops the batch and undoes everything
// already written: -stripped files are deleted; in-place originals are
// restored from `.handy-bak` sidecars. Rollback is best-effort — a step
// that itself fails surfaces a Failure with Code ROLLBACK_FAILED.
//
// IMPORTANT: in --in-place mode a hard-kill (SIGKILL, power loss) mid-batch
// leaves `<source>.handy-bak` orphans next to processed files. The names
// are stable so a follow-up sweep can clean them up.
func Run(ctx context.Context, req Request) <-chan tools.Progress {
	ch := make(chan tools.Progress, 8)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "strip-meta"
			p.Action = "run"
			p.StartedAt = started
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		if len(req.Sources) == 0 {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeBadRequest, Message: "strip-meta needs at least one source",
			}})
			return
		}

		var (
			processed   int
			failures    []tools.Failure
			rb          tools.RollbackStack
			aborted     bool
			pendingBaks []string // .handy-bak files to clean up on batch success
		)
		total := len(req.Sources)
		opts := image.Options{Quality: req.Quality, StripMetadata: true}

		recordFailure := func(src, name, code, msg string, i int, terr *tools.Error) {
			failures = append(failures, tools.Failure{Path: src, Code: code, Message: msg})
			ev := tools.Progress{
				Level: tools.SeverityError, CurrentItem: name,
				Fraction: float64(i+1) / float64(total),
				Message:  fmt.Sprintf("[%d/%d] %s: %s (%s)", i+1, total, name, msg, code),
			}
			if terr != nil {
				ev.Err = terr
			}
			emit(ev)
		}

		for i, src := range req.Sources {
			if err := ctx.Err(); err != nil {
				emit(tools.Progress{Completed: true, Err: &tools.Error{
					Code: tools.CodeAborted, Message: "strip-meta canceled",
				}, Failures: failures})
				return
			}
			name := filepath.Base(src)
			target, ok := detectImageFormat(src)
			if !ok {
				recordFailure(src, name, tools.CodeUnsupportedInput, "unrecognised image extension", i, nil)
				if req.Rollback {
					aborted = true
					break
				}
				continue
			}

			outPath := strippedOutputPath(src, req.InPlace)
			if req.InPlace && req.Rollback {
				bak := src + ".handy-bak"
				if err := copyFile(src, bak); err != nil {
					recordFailure(src, name, tools.ClassifyFSError(err),
						"snapshot for rollback failed: "+err.Error(), i, nil)
					aborted = true
					break
				}
				bakCopy, srcCopy := bak, src
				rb.Push(tools.RollbackStep{
					Undo: func() error { return os.Rename(bakCopy, srcCopy) },
					Note: bakCopy + " -> " + srcCopy,
				})
				pendingBaks = append(pendingBaks, bak)
			}

			if terr := convertOne(ctx, src, outPath, target, opts); terr != nil {
				recordFailure(src, name, terr.Code, terr.Message, i, terr)
				if req.Rollback {
					aborted = true
					break
				}
				continue
			}

			processed++
			if req.Rollback && !req.InPlace {
				written := outPath
				rb.Push(tools.RollbackStep{
					Undo: func() error { return os.Remove(written) },
					Note: "remove " + written,
				})
			}
			emit(tools.Progress{
				Level: tools.SeverityInfo, CurrentItem: name,
				Fraction: float64(i+1) / float64(total),
				Message:  fmt.Sprintf("[%d/%d] %s", i+1, total, name),
			})
		}

		if aborted {
			undoFailures := rb.Replay()
			failures = append(failures, undoFailures...)
			for _, uf := range undoFailures {
				emit(tools.Progress{Level: tools.SeverityError,
					Message: fmt.Sprintf("rollback: %s: %s", uf.Path, uf.Message)})
			}
			code := tools.CodeIO
			if len(failures) > 0 {
				code = failures[0].Code
			}
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: code, Message: "strip-meta aborted with rollback",
			}, Failures: failures})
			return
		}

		// Batch fully (or partially) succeeded: drop every snapshot we held
		// for potential rollback so we don't leave .handy-bak orphans next
		// to the cleaned files.
		for _, bak := range pendingBaks {
			_ = os.Remove(bak)
		}

		summary := fmt.Sprintf("stripped %d/%d image(s)", processed, total)
		if len(failures) > 0 && processed == 0 {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CoalesceFailureCode(failures), Message: "all strip-meta operations failed", Detail: summary,
			}, Failures: failures})
			return
		}
		emit(tools.Progress{Completed: true, Fraction: 1, Level: tools.SeverityInfo,
			Message: summary, Failures: failures})
	}()
	return ch
}

// detectImageFormat maps a source path's extension to image.Format. Mirrors
// the CLI's parseImageFormat vocabulary so the user's `--format` choices
// (png, jpg, jpeg, bmp, gif, tiff, webp, heic) line up with what strip-meta
// will accept. webp/heic are not supported here because their encoders
// require the magick binary; that's surfaced as UNSUPPORTED_INPUT.
func detectImageFormat(p string) (image.Format, bool) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(p)), ".")
	switch ext {
	case "png":
		return image.FormatPNG, true
	case "jpg", "jpeg":
		return image.FormatJPEG, true
	case "bmp":
		return image.FormatBMP, true
	case "gif":
		return image.FormatGIF, true
	case "tif", "tiff":
		return image.FormatTIFF, true
	}
	return image.FormatUnspecified, false
}

// strippedOutputPath returns `<base>-stripped<ext>` next to the source, or
// the source path verbatim when inPlace is true (so the underlying
// image.Convert overwrites it).
func strippedOutputPath(src string, inPlace bool) string {
	if inPlace {
		return src
	}
	ext := filepath.Ext(src)
	return strings.TrimSuffix(src, ext) + "-stripped" + ext
}

// convertOne wraps image.Convert in a synchronous drain because stripmeta's
// outer loop is sequential and we want a single tools.Error back, not a
// channel. Returns nil on success.
func convertOne(ctx context.Context, src, outPath string, target image.Format, opts image.Options) *tools.Error {
	ch := image.Convert(ctx, image.ConvertRequest{
		Source:       src,
		TargetFormat: target,
		Opts:         opts,
		Output:       outPath,
		Overwrite:    true,
	})
	var last tools.Progress
	for ev := range ch {
		last = ev
	}
	return last.Err
}

// copyFile is a small helper used to snapshot an in-place source to its
// .handy-bak sidecar before destructive overwrite. Uses os.ReadFile +
// os.WriteFile because the typical image is well under a megabyte and the
// simplicity outweighs the buffered-IO savings; for large TIFFs this still
// fits in RAM comfortably given users are operating one-at-a-time.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src) //nolint:gosec // caller-supplied path
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644) //nolint:gosec // mirror typical user-owned image file mode
}
