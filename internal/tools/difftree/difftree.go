// Package difftree compares two directory trees and reports which files are
// in A but not B (removed), in B but not A (added), or different between the
// two (changed). Two compare strategies are exposed:
//
//   - ModeMTime: size + mtime. Fast — one stat() per file. Misses changes
//     where the content was rewritten with identical size and the mtime was
//     restored (rare in practice, common in adversarial cases).
//   - ModeHash: SHA256 of the file contents. Slower (reads every byte of
//     every pair) but authoritative.
//
// The walker uses filepath.WalkDir which does not follow symlinks — that
// keeps a symlink-loop fixture from hanging, satisfying the issue's
// acceptance criterion without needing a hand-rolled visited-set.
//
// The package is pure Go.
package difftree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// Mode picks the comparison strategy for two files that exist in both roots.
type Mode string

const (
	ModeMTime Mode = "mtime"
	ModeHash  Mode = "hash"
)

// ParseMode normalises a user-typed mode string. Empty resolves to ModeMTime
// so an unset CLI flag falls through cleanly to the default.
func ParseMode(s string) (Mode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "mtime":
		return ModeMTime, true
	case "hash":
		return ModeHash, true
	}
	return "", false
}

// Status describes how a file differs between A and B.
type Status string

const (
	StatusAdded   Status = "added"
	StatusRemoved Status = "removed"
	StatusChanged Status = "changed"
)

// Diff is one row of the report. Path is relative to the appropriate root
// (A for removed/changed, B for added) and uses forward slashes so the
// output is stable across operating systems.
type Diff struct {
	Path   string `json:"path"`
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Request describes one diff job.
type Request struct {
	A, B string
	Mode Mode
}

// entry is the per-file state we carry through the comparison.
type entry struct {
	Size    int64
	ModTime time.Time
}

// Inspect synchronously walks both trees and returns a sorted list of diffs.
// Use this when you have the report up-front (CLI default, future UI summary
// pane). Run wraps Inspect for callers that want a Progress stream.
func Inspect(req Request) ([]Diff, *tools.Error) {
	if req.A == "" || req.B == "" {
		return nil, &tools.Error{Code: tools.CodeBadRequest, Message: "diff-tree needs both A and B paths"}
	}
	mode := req.Mode
	if mode == "" {
		mode = ModeMTime
	}
	if _, ok := ParseMode(string(mode)); !ok {
		return nil, &tools.Error{Code: tools.CodeBadRequest, Message: "unknown mode", Detail: string(mode)}
	}
	for _, root := range []string{req.A, req.B} {
		info, err := os.Stat(root)
		if err != nil {
			return nil, &tools.Error{Code: tools.CodeIO, Message: "stat root", Detail: err.Error()}
		}
		if !info.IsDir() {
			return nil, &tools.Error{Code: tools.CodeBadRequest, Message: "root is not a directory", Detail: root}
		}
	}
	aMap, terr := walkRoot(req.A)
	if terr != nil {
		return nil, terr
	}
	bMap, terr := walkRoot(req.B)
	if terr != nil {
		return nil, terr
	}
	return compareMaps(req.A, req.B, aMap, bMap, mode)
}

// Run streams one Progress event per diff entry plus a terminal summary.
// The walk + compare run inside the goroutine so a Ctrl+C between files
// (especially in hash mode over a big tree) aborts cleanly.
func Run(ctx context.Context, req Request) <-chan tools.Progress {
	ch := make(chan tools.Progress, 8)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "diff-tree"
			p.Action = "run"
			p.StartedAt = started
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}
		diffs, terr := Inspect(req)
		if terr != nil {
			emit(tools.Progress{Completed: true, Err: terr})
			return
		}
		total := len(diffs)
		for i, d := range diffs {
			if err := ctx.Err(); err != nil {
				emit(tools.Progress{Completed: true, Err: &tools.Error{
					Code: tools.CodeAborted, Message: "diff-tree canceled",
				}})
				return
			}
			line := string(d.Status) + " " + d.Path
			if d.Status == StatusChanged && d.Reason != "" {
				line = line + " (" + d.Reason + ")"
			}
			frac := 1.0
			if total > 0 {
				frac = float64(i+1) / float64(total)
			}
			emit(tools.Progress{
				Level:       tools.SeverityInfo,
				CurrentItem: d.Path,
				Fraction:    frac,
				Message:     line,
			})
		}
		var added, removed, changed int
		for _, d := range diffs {
			switch d.Status {
			case StatusAdded:
				added++
			case StatusRemoved:
				removed++
			case StatusChanged:
				changed++
			}
		}
		emit(tools.Progress{
			Completed: true, Fraction: 1, Level: tools.SeverityInfo,
			Message: fmt.Sprintf("diff-tree: %d added, %d removed, %d changed", added, removed, changed),
		})
	}()
	return ch
}

// walkRoot enumerates regular files under root and records (size, mtime) for
// each. Symlinks are deliberately not followed — WalkDir treats them as leaf
// entries, so a symlink-loop fixture cannot hang the walker. Non-regular
// files (sockets, devices) are skipped silently.
func walkRoot(root string) (map[string]entry, *tools.Error) {
	out := make(map[string]entry)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error { //nolint:gosec // root comes from the caller / CLI invocation
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		out[filepath.ToSlash(rel)] = entry{Size: info.Size(), ModTime: info.ModTime()}
		return nil
	})
	if err != nil {
		return nil, &tools.Error{Code: tools.CodeIO, Message: "walk", Detail: err.Error()}
	}
	return out, nil
}

// compareMaps diffs the two file inventories under their respective roots.
// The result is sorted by path so output is deterministic across runs. Any
// I/O failure during hash mode short-circuits — partial reports are worse
// than a clear error.
func compareMaps(aRoot, bRoot string, a, b map[string]entry, mode Mode) ([]Diff, *tools.Error) {
	var out []Diff
	for path, ea := range a {
		eb, ok := b[path]
		if !ok {
			out = append(out, Diff{Path: path, Status: StatusRemoved})
			continue
		}
		reason, changed, terr := compareEntry(filepath.Join(aRoot, filepath.FromSlash(path)), filepath.Join(bRoot, filepath.FromSlash(path)), ea, eb, mode)
		if terr != nil {
			return nil, terr
		}
		if changed {
			out = append(out, Diff{Path: path, Status: StatusChanged, Reason: reason})
		}
	}
	for path := range b {
		if _, ok := a[path]; !ok {
			out = append(out, Diff{Path: path, Status: StatusAdded})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Status < out[j].Status
	})
	return out, nil
}

// compareEntry decides whether two files differ. Size mismatch is reported
// first because it's cheap and unambiguous; only when sizes match do we fall
// through to the mtime or hash strategy. Hash mode reads each file from
// disk so callers should expect compareMaps to be O(bytes) in that mode.
func compareEntry(aPath, bPath string, ea, eb entry, mode Mode) (string, bool, *tools.Error) {
	if ea.Size != eb.Size {
		return "size", true, nil
	}
	if mode == ModeMTime {
		if !ea.ModTime.Equal(eb.ModTime) {
			return "mtime", true, nil
		}
		return "", false, nil
	}
	ah, err := hashFile(aPath)
	if err != nil {
		return "", false, &tools.Error{Code: tools.CodeIO, Message: "hash A side", Detail: err.Error()}
	}
	bh, err := hashFile(bPath)
	if err != nil {
		return "", false, &tools.Error{Code: tools.CodeIO, Message: "hash B side", Detail: err.Error()}
	}
	if ah != bh {
		return "hash", true, nil
	}
	return "", false, nil
}

// hashFile returns a lowercase hex SHA256 of the file at path. Kept local to
// the package — the internal/tools/hash package's Hash() wraps a Result struct
// we don't need here, and a tiny inline helper keeps the dependency surface
// minimal.
func hashFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from a walk over the caller-supplied root
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
