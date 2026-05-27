// Package hash computes file digests for verification and integrity checks.
// Supports stdlib crypto/md5, crypto/sha256, and lukechampine.com/blake3
// (pure-Go BLAKE3). Two operations:
//
//   - Run: hash a slice of files, stream one Progress event per file plus a
//     terminal summary. Each event's Message carries the digest in the
//     traditional `sha256sum`-style format (`<digest>  <path>`).
//   - Verify: read a manifest file (same sha256sum format), recompute the
//     digest for each entry, report OK / FAILED per row. The terminal event
//     carries the matched / mismatched counts.
//
// The package is pure Go — no system binaries.
package hash

import (
	"bufio"
	"context"
	"crypto/md5" //nolint:gosec // MD5 here is a checksum, not a security primitive — common manifest format
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lukechampine.com/blake3"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// Algo names the hash algorithm. Used by both Run and Verify; the string form
// also matches the algorithm-prefix some manifest tools emit (e.g. "MD5 (path)").
type Algo string

const (
	AlgoMD5    Algo = "md5"
	AlgoSHA256 Algo = "sha256"
	AlgoBLAKE3 Algo = "blake3"
)

// ParseAlgo maps a user-typed string to an Algo, normalising case. Returns
// false for unrecognised values.
func ParseAlgo(s string) (Algo, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "md5":
		return AlgoMD5, true
	case "sha256":
		return AlgoSHA256, true
	case "blake3":
		return AlgoBLAKE3, true
	}
	return "", false
}

// newHasher returns a fresh hash.Hash for the requested algo. Unknown algos
// return a BadRequest tools.Error; callers should normally have validated
// via ParseAlgo first.
func newHasher(a Algo) (hash.Hash, *tools.Error) {
	switch a {
	case AlgoMD5:
		return md5.New(), nil //nolint:gosec // not a security primitive — see package doc
	case AlgoSHA256:
		return sha256.New(), nil
	case AlgoBLAKE3:
		return blake3.New(32, nil), nil
	}
	return nil, &tools.Error{
		Code:    tools.CodeBadRequest,
		Message: "unknown algo",
		Detail:  string(a),
	}
}

// Request describes a Run batch.
type Request struct {
	Sources []string
	Algo    Algo

	// Parallelism caps the number of concurrent file-hashing goroutines.
	// 0 (the default) auto-sizes to runtime.GOMAXPROCS(0); 1 forces serial
	// — useful in tests that depend on per-file event ordering. Hash
	// computation is CPU-bound (md5/sha256) or memory-bandwidth-bound
	// (blake3) so parallelism scales near-linearly until disk becomes the
	// bottleneck.
	Parallelism int
}

// Result is the digest + path for one file. Surfaced both on the channel
// (inside Progress.Message in the traditional format) and as a struct that
// the CLI's --json output can encode directly.
type Result struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Algo   Algo   `json:"algo"`
}

// Run hashes Sources concurrently, streaming one Progress event per file
// plus a terminal summary. The per-file event's Message holds the
// `<digest>  <path>` line so a `--quiet` CLI can still capture it via the
// terminal event's progress aggregation.
//
// Per-file events arrive in worker-completion order, not Sources order,
// whenever Parallelism != 1. The terminal event is always last. Tests must
// assert on (set-of-events, terminal-shape) rather than positional ordering.
func Run(ctx context.Context, req Request) <-chan tools.Progress {
	ch := make(chan tools.Progress, 8)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "hash"
			p.Action = "run"
			p.StartedAt = started
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}
		if len(req.Sources) == 0 {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeBadRequest, Message: "hash needs at least one source",
			}})
			return
		}
		if _, terr := newHasher(req.Algo); terr != nil {
			emit(tools.Progress{Completed: true, Err: terr})
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

		jobs := make(chan string)
		var (
			wg        sync.WaitGroup
			completed atomic.Int64
			failed    atomic.Int64
		)

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for src := range jobs {
					if err := ctx.Err(); err != nil {
						return
					}
					res, terr := Hash(ctx, src, req.Algo)
					done := completed.Add(1)
					if terr != nil {
						failed.Add(1)
						emit(tools.Progress{
							Level:       tools.SeverityError,
							CurrentItem: filepath.Base(src),
							Fraction:    float64(done) / float64(total),
							Message:     fmt.Sprintf("[%d/%d] %s: %s (%s)", done, total, src, terr.Message, terr.Code),
							Err:         terr,
						})
						continue
					}
					emit(tools.Progress{
						Level:       tools.SeverityInfo,
						CurrentItem: filepath.Base(src),
						Fraction:    float64(done) / float64(total),
						Message:     fmt.Sprintf("%s  %s", res.Digest, res.Path),
					})
				}
			}()
		}

	feed:
		for _, src := range req.Sources {
			select {
			case <-ctx.Done():
				break feed
			case jobs <- src:
			}
		}
		close(jobs)
		wg.Wait()

		if err := ctx.Err(); err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeAborted, Message: "hash canceled",
			}})
			return
		}
		nFailed := int(failed.Load())
		summary := fmt.Sprintf("hashed %d/%d file(s) with %s", total-nFailed, total, req.Algo)
		if nFailed == total {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: "all hashes failed", Detail: summary,
			}})
			return
		}
		emit(tools.Progress{
			Completed: true, Fraction: 1, Level: tools.SeverityInfo,
			Message: summary,
		})
	}()
	return ch
}

// Hash computes the digest of a single file using algo. Returns a structured
// Result on success. Used by Run internally and exported so callers (CLI,
// Verify, future server) can hash one file without spinning up a channel.
//
// The ctx parameter is checked between 32 KiB read chunks of the file's
// stream, so a Ctrl+C during a multi-GB hash preempts within a chunk
// instead of blocking on the whole file (the old io.Copy was uncancellable).
// Pass context.Background() if you don't need cancellation.
func Hash(ctx context.Context, path string, algo Algo) (*Result, *tools.Error) {
	h, terr := newHasher(algo)
	if terr != nil {
		return nil, terr
	}
	f, err := os.Open(path) //nolint:gosec // caller-supplied path; CLI / HTTP layer enforces path safety
	if err != nil {
		return nil, &tools.Error{Code: tools.ClassifyFSError(err), Message: "open file", Detail: err.Error()}
	}
	defer f.Close()
	if terr := streamInto(ctx, h, f); terr != nil {
		return nil, terr
	}
	return &Result{
		Path:   path,
		Digest: hex.EncodeToString(h.Sum(nil)),
		Algo:   algo,
	}, nil
}

// streamCopyChunk is the read-and-write granularity used by streamInto.
// 32 KiB matches io.Copy's default buffer and keeps memory flat across
// many-file workloads — a 500-file batch with a 1 MiB chunk would burn
// 500 MiB of GC pressure. ctx.Err() is checked between chunks, so a 1 GB
// hash preempts within ~100 µs (32 KiB / typical 300 MiB/s sha256).
const streamCopyChunk = 32 << 10

// streamInto copies r into h in streamCopyChunk-sized chunks, checking
// ctx.Err() between chunks. Used by Hash so a Run worker that takes ctx
// from the caller can preempt a long-running file mid-hash.
func streamInto(ctx context.Context, h hash.Hash, r io.Reader) *tools.Error {
	buf := make([]byte, streamCopyChunk)
	for {
		if err := ctx.Err(); err != nil {
			return &tools.Error{Code: tools.CodeAborted, Message: "hash canceled", Detail: err.Error()}
		}
		n, err := r.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF { //nolint:errorlint // io.Reader contract guarantees the sentinel here
			return nil
		}
		if err != nil {
			return &tools.Error{Code: tools.ClassifyFSError(err), Message: "read file", Detail: err.Error()}
		}
	}
}

// VerifyEntry is one row of a parsed manifest plus its recomputed status.
type VerifyEntry struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Got      string `json:"got,omitempty"`
	OK       bool   `json:"ok"`
	Err      string `json:"err,omitempty"`
}

// Verify reads a sha256sum-format manifest and recomputes each entry's
// digest. The manifest's algorithm is implicit (callers pass it); the
// canonical sha256sum file format is `<hex-digest>  <path>` per line where
// the separator is exactly two spaces. Lines starting with `#` and blank
// lines are skipped. Returns one VerifyEntry per non-blank row.
//
// Errors:
//   - Unreadable manifest → IO_ERROR.
//   - Malformed line → BAD_REQUEST (preserved line number in Detail).
//   - Each unreadable target file is reported per-entry via OK=false + Err,
//     not as a Verify-level failure — so a manifest with some missing files
//     still produces a usable report.
func Verify(ctx context.Context, manifestPath string, algo Algo) ([]VerifyEntry, *tools.Error) {
	if _, terr := newHasher(algo); terr != nil {
		return nil, terr
	}
	f, err := os.Open(manifestPath) //nolint:gosec // caller-supplied path
	if err != nil {
		return nil, &tools.Error{Code: tools.ClassifyFSError(err), Message: "open manifest", Detail: err.Error()}
	}
	defer f.Close()

	manifestDir := filepath.Dir(manifestPath)
	var entries []VerifyEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<16), 1<<20)
	for line := 1; scanner.Scan(); line++ {
		row := scanner.Text()
		if row == "" || strings.HasPrefix(row, "#") {
			continue
		}
		digest, path, ok := parseManifestLine(row)
		if !ok {
			return nil, &tools.Error{
				Code:    tools.CodeBadRequest,
				Message: "malformed manifest line",
				Detail:  fmt.Sprintf("line %d: %q", line, row),
			}
		}
		// Paths in a sha256sum manifest are typically relative to the
		// manifest's directory; resolve before hashing so a user can run
		// `htools hash --check SHA256SUMS` from anywhere.
		target := path
		if !filepath.IsAbs(target) {
			target = filepath.Join(manifestDir, path)
		}
		if err := ctx.Err(); err != nil {
			return nil, &tools.Error{Code: tools.CodeAborted, Message: "verify canceled", Detail: err.Error()}
		}
		entry := VerifyEntry{Path: path, Expected: digest}
		res, terr := Hash(ctx, target, algo)
		if terr != nil {
			entry.OK = false
			entry.Err = terr.Message
			entries = append(entries, entry)
			continue
		}
		entry.Got = res.Digest
		entry.OK = strings.EqualFold(res.Digest, digest)
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, &tools.Error{Code: tools.ClassifyFSError(err), Message: "read manifest", Detail: err.Error()}
	}
	return entries, nil
}

// parseManifestLine splits a `<digest>  <path>` line. The canonical separator
// is two spaces, but some emitters use a single space — accept both. A
// leading `*` on the path (binary mode marker, traditional in coreutils
// hashers) is stripped. The leading token must look like a hex digest
// (32 chars for MD5, 64 for SHA256 / BLAKE3) — that's how a malformed line
// is rejected without needing the algorithm passed in.
func parseManifestLine(s string) (digest, path string, ok bool) {
	candidate := func(d, p string) (string, string, bool) {
		if !looksLikeHexDigest(d) {
			return "", "", false
		}
		return d, strings.TrimPrefix(strings.TrimSpace(p), "*"), true
	}
	// Try two-space separator first (canonical).
	if i := strings.Index(s, "  "); i > 0 {
		if d, p, ok := candidate(s[:i], s[i+2:]); ok {
			return d, p, true
		}
	}
	// Fall back to single space / tab.
	if i := strings.IndexAny(s, " \t"); i > 0 && i < len(s)-1 {
		if d, p, ok := candidate(s[:i], s[i+1:]); ok {
			return d, p, true
		}
	}
	return "", "", false
}

// looksLikeHexDigest reports whether s is non-empty, even-length, and
// composed only of hex digits. Doesn't check the length matches a specific
// algorithm — the manifest can be from any of MD5 (32), SHA256 (64), or
// BLAKE3 (64), and Verify enforces the chosen algorithm separately when it
// recomputes.
func looksLikeHexDigest(s string) bool {
	if s == "" || len(s)%2 != 0 || len(s) < 32 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
