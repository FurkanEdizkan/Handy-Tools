package archive

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/flate"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dsnet/compress/bzip2"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// CompressRequest describes an archive-creation job. It mirrors the proto
// handytools.v1.CompressRequest.
type CompressRequest struct {
	Sources          []string // files and/or directories to pack
	Format           Format   // target format; FormatUnknown -> inferred from Output extension
	Output           string   // path of the archive to create
	Password         string   // optional; rejected here (the pure-Go formats have no encryption)
	CompressionLevel int      // 0 = format default; 1 = fastest .. 9 = smallest
	Overwrite        bool     // true: replace an existing file at Output; false (default): rename to Output's stem + "-1", "-2", ... like image convert
}

// CompressInspection is the result of InspectCompress.
type CompressInspection struct {
	Format     Format            // resolved format (from req.Format or req.Output extension)
	EntryCount int               // best-effort count of files that would be packed
	Issues     []tools.PathIssue // preflight: sources missing/unreadable, output dir unwritable
}

// InspectCompress is the dry-run / preflight for Compress. It verifies that
// every source can be stat'd and that the output directory accepts writes,
// returning a structured Issues slice. It does not open the output file or
// pack anything. Callers use this for `--dry-run` and `--strict` flows.
func InspectCompress(req CompressRequest) (CompressInspection, *tools.Error) {
	if len(req.Sources) == 0 {
		return CompressInspection{}, &tools.Error{Code: tools.CodeBadRequest, Message: "no sources to compress"}
	}
	if req.Output == "" {
		return CompressInspection{}, &tools.Error{Code: tools.CodeBadRequest, Message: "no output path given"}
	}
	ins := CompressInspection{Format: req.Format}
	if ins.Format == FormatUnknown {
		ins.Format = detectFormat(req.Output)
	}
	ins.Issues = append(ins.Issues, tools.StatInputs(req.Sources)...)
	if issue := tools.CheckOutputDirWritable(filepath.Dir(req.Output)); issue != nil {
		ins.Issues = append(ins.Issues, *issue)
	}
	// EntryCount is best-effort: if every source stats cleanly, walk them to
	// get the actual file count. If anything failed, leave it at zero so the
	// caller doesn't make claims it can't back up.
	if len(ins.Issues) == 0 {
		if entries, err := gatherEntries(req.Sources); err == nil {
			ins.EntryCount = len(entries)
		}
	}
	return ins, nil
}

// Compress packs Sources into a single archive at req.Output. zip, tar,
// tar.gz, tar.bz2 and tar.zst are produced in pure Go; directory sources are
// added recursively. RAR creation is unsupported (proprietary); 7z creation
// is delegated to the system "7z" binary when present.
func Compress(ctx context.Context, req CompressRequest) <-chan tools.Progress {
	ch := make(chan tools.Progress, 8)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "archive"
			p.Action = "compress"
			p.StartedAt = started
			if p.CurrentItem == "" {
				p.CurrentItem = filepath.Base(req.Output)
			}
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		if len(req.Sources) == 0 {
			emit(compressFail(tools.CodeBadRequest, "no sources to compress", ""))
			return
		}
		if req.Output == "" {
			emit(compressFail(tools.CodeBadRequest, "no output path given", ""))
			return
		}
		if req.Password != "" {
			emit(compressFail(tools.CodeUnsupportedInput,
				"password-protected archive creation is not supported",
				"zip/tar/gz/bz2/zstd have no encryption in the pure-Go path"))
			return
		}
		if req.Format == FormatUnknown {
			req.Format = detectFormat(req.Output)
		}

		entries, err := gatherEntries(req.Sources)
		if err != nil {
			emit(compressFail(tools.ClassifyFSError(err), "scan sources", err.Error()))
			return
		}
		if len(entries) == 0 {
			emit(compressFail(tools.CodeBadRequest, "sources contain no files", ""))
			return
		}

		emit(tools.Progress{Level: tools.SeverityInfo,
			Message: fmt.Sprintf("packing %d file(s) into %s", len(entries), req.Format)})

		// Write to <output>.partial first, then atomically rename on success.
		// This guarantees the user never sees a half-written archive at the
		// final path: either it's there in full, or it isn't. On any failure
		// the .partial file is removed so retries don't see a stale stub.
		// 7z is delegated to the external binary but we still hand it the
		// .partial path so the pattern is uniform across formats.
		finalOutput := req.Output
		stagedReq := req
		stagedReq.Output = finalOutput + ".partial"
		_ = os.Remove(stagedReq.Output) // pre-clean any leftover from an aborted run

		switch req.Format {
		case FormatRar:
			emit(compressFail(tools.CodeUnsupportedInput, "cannot create RAR archives",
				"RAR is proprietary; no creation tool is bundled"))
			return
		case FormatSevenZ:
			err = compressSevenZ(ctx, stagedReq, emit)
		case FormatZip:
			err = writeZip(ctx, stagedReq, entries, emit)
		case FormatTar, FormatTarGz, FormatTarBz2, FormatTarZst, FormatTarXz:
			err = writeTarArchive(ctx, stagedReq, entries, emit)
		default:
			emit(compressFail(tools.CodeUnsupportedInput, "unsupported archive format",
				req.Format.String()))
			return
		}
		if err != nil {
			_ = os.Remove(stagedReq.Output)
			var terr *tools.Error
			if errors.As(err, &terr) {
				emit(tools.Progress{Completed: true, Err: terr})
			} else {
				emit(compressFail(tools.ClassifyFSError(err), "compress failed", err.Error()))
			}
			return
		}
		// Collision avoidance: when Overwrite=false, pick a fresh name before
		// the rename clobbers an existing file. NextUniquePath stat-loops
		// without touching the filesystem; safe here because the actual
		// bytes-on-disk creation is the atomic Rename below.
		if !req.Overwrite {
			unique, perr := tools.NextUniquePath(finalOutput)
			if perr != nil {
				_ = os.Remove(stagedReq.Output)
				emit(compressFail(tools.ClassifyFSError(perr), "cannot pick unique output path", perr.Error()))
				return
			}
			finalOutput = unique
		}
		if err := os.Rename(stagedReq.Output, finalOutput); err != nil {
			_ = os.Remove(stagedReq.Output)
			emit(compressFail(tools.ClassifyFSError(err), "finalize archive", err.Error()))
			return
		}
		emit(tools.Progress{Completed: true, Fraction: 1, Level: tools.SeverityInfo,
			Message: "wrote " + finalOutput})
	}()
	return ch
}

func compressFail(code, msg, detail string) tools.Progress {
	return tools.Progress{Completed: true, Err: &tools.Error{Code: code, Message: msg, Detail: detail}}
}

// compressEntry is one file destined for the archive.
type compressEntry struct {
	abs  string      // absolute filesystem path
	name string      // slash-separated path inside the archive
	info os.FileInfo // for mode + mtime
}

// gatherEntries expands the source list into a flat file list. A directory
// source is walked recursively; its entries are named relative to the
// directory's parent, so "/data/logs" yields "logs/2026/app.log" etc.
func gatherEntries(sources []string) ([]compressEntry, error) {
	var out []compressEntry
	for _, src := range sources {
		info, err := os.Stat(src)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, compressEntry{abs: src, name: filepath.Base(src), info: info})
			continue
		}
		root := filepath.Clean(src)
		parent := filepath.Dir(root)
		walkErr := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.IsDir() {
				return nil // files only — extractors recreate parent dirs
			}
			rel, err := filepath.Rel(parent, p)
			if err != nil {
				return err
			}
			out = append(out, compressEntry{abs: p, name: filepath.ToSlash(rel), info: fi})
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	return out, nil
}

func writeZip(ctx context.Context, req CompressRequest, entries []compressEntry, emit emitFn) error {
	out, err := createOutput(req.Output)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	if lvl, custom := flateLevel(req.CompressionLevel); custom {
		zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(w, lvl)
		})
	}
	totalBytes := sumEntryBytes(entries)
	var done int64
	for i, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Deflate, Modified: e.info.ModTime()}
		hdr.SetMode(e.info.Mode())
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		if err := packEntry(ctx, emit, w, e, &done, totalBytes, i, len(entries)); err != nil {
			return err
		}
	}
	return zw.Close()
}

func writeTarArchive(ctx context.Context, req CompressRequest, entries []compressEntry, emit emitFn) error {
	out, err := createOutput(req.Output)
	if err != nil {
		return err
	}
	defer out.Close()

	// Build the compression layer (if any) wrapping the output file. The
	// layers must be closed innermost-first, before the file, to flush.
	var w io.Writer = out
	var closers []io.Closer
	switch req.Format {
	case FormatTarGz:
		gz, err := gzip.NewWriterLevel(out, gzipLevel(req.CompressionLevel))
		if err != nil {
			return err
		}
		w, closers = gz, append(closers, gz)
	case FormatTarBz2:
		bz, err := bzip2.NewWriter(out, &bzip2.WriterConfig{Level: bzip2Level(req.CompressionLevel)})
		if err != nil {
			return err
		}
		w, closers = bz, append(closers, bz)
	case FormatTarZst:
		zw, err := zstd.NewWriter(out, zstd.WithEncoderLevel(zstdLevel(req.CompressionLevel)))
		if err != nil {
			return err
		}
		w, closers = zw, append(closers, zw)
	case FormatTarXz:
		// ulikunitz/xz has no simple 0..9 preset knob; use the library default.
		xw, err := xz.NewWriter(out)
		if err != nil {
			return err
		}
		w, closers = xw, append(closers, xw)
	}

	tw := tar.NewWriter(w)
	totalBytes := sumEntryBytes(entries)
	var done int64
	for i, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(e.info, "")
		if err != nil {
			return err
		}
		hdr.Name = e.name
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if err := packEntry(ctx, emit, tw, e, &done, totalBytes, i, len(entries)); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	for j := len(closers) - 1; j >= 0; j-- {
		if err := closers[j].Close(); err != nil {
			return err
		}
	}
	return nil
}

// compressSevenZ delegates 7z creation to the system "7z" binary.
func compressSevenZ(ctx context.Context, req CompressRequest, emit emitFn) error {
	r := sysdep.Lookup("7z")
	if !r.Found {
		return &tools.Error{
			Code:    tools.CodeMissingBinary,
			Message: "7z not found on PATH",
			Detail:  "install hint: " + r.Tool.InstallHint["linux"],
		}
	}
	args := append([]string{"a", req.Output}, req.Sources...)
	cmd := exec.CommandContext(ctx, r.UsedAlias, args...) //nolint:gosec
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}

	// 7z doesn't report parseable progress; animate an ETA from the total
	// input size while it compresses. The caller's terminal event finishes it.
	stop := tools.RunEstimator(ctx, func(p tools.Progress) { emit(p) },
		tools.Estimator{Tool: "archive", Action: "compress-7z", InputSize: sumSourceBytes(req.Sources), Start: time.Now()},
		tools.Progress{Level: tools.SeverityInfo})
	defer stop()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		emit(tools.Progress{Level: tools.SeverityInfo, Message: scanner.Text()})
	}
	return cmd.Wait()
}

// sumSourceBytes returns the best-effort total size of the given source paths
// (recursing into directories). It's used only to seed the ETA estimator, so
// unreadable entries are skipped rather than erroring.
func sumSourceBytes(sources []string) int64 {
	var total int64
	for _, s := range sources {
		_ = filepath.WalkDir(s, func(_ string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil //nolint:nilerr // best-effort sizing; skip unreadable
			}
			if info, statErr := d.Info(); statErr == nil {
				total += info.Size()
			}
			return nil
		})
	}
	return total
}

func createOutput(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.Create(path) //nolint:gosec // caller-controlled output path
}

// sumEntryBytes totals the uncompressed sizes of all entries — the denominator
// for byte-weighted pack progress.
func sumEntryBytes(entries []compressEntry) int64 {
	var t int64
	for _, e := range entries {
		t += e.info.Size()
	}
	return t
}

// packEntry streams one source file into the archive writer while reporting
// byte-weighted progress. *done is the running total of bytes packed across
// prior entries; a Ticker reports (done + this entry's counter) / totalBytes so
// the bar advances even mid-entry for a single large file. On return it
// advances *done and emits the per-entry summary line.
func packEntry(ctx context.Context, emit emitFn, dst io.Writer, e compressEntry, done *int64, totalBytes int64, idx, count int) error {
	f, err := os.Open(e.abs) //nolint:gosec // path comes from the caller's source list
	if err != nil {
		return err
	}
	defer f.Close()

	cr := tools.NewCountingReader(f)
	base := *done
	stop := tools.Ticker(ctx, func(p tools.Progress) { emit(p) }, 0,
		func() (float64, int64, int64) {
			cur := base + cr.Count()
			frac := 0.0
			if totalBytes > 0 {
				frac = float64(cur) / float64(totalBytes)
			}
			return frac, cur, totalBytes
		},
		tools.Progress{Level: tools.SeverityInfo, CurrentItem: e.name})
	n, err := io.Copy(dst, cr) //nolint:gosec // size-bounded by the source file
	stop()
	if err != nil {
		return err
	}

	*done += n
	frac := 0.0
	if totalBytes > 0 {
		frac = float64(*done) / float64(totalBytes)
	}
	emit(tools.Progress{
		Level:       tools.SeverityInfo,
		Message:     fmt.Sprintf("[%d/%d] %s", idx+1, count, e.name),
		Fraction:    frac,
		BytesDone:   *done,
		BytesTotal:  totalBytes,
		CurrentItem: e.name,
	})
	return nil
}

// ---- compression-level mapping ---------------------------------------------
// Requests use a uniform 0..9 scale (0 = format default). Each format maps it
// onto its own native scale.

func clampLevel(n int) int {
	switch {
	case n < 0:
		return 0
	case n > 9:
		return 9
	default:
		return n
	}
}

// flateLevel reports the flate level for zip; custom is false for level 0,
// meaning "leave zip on its default Deflate compressor".
func flateLevel(n int) (level int, custom bool) {
	n = clampLevel(n)
	if n == 0 {
		return 0, false
	}
	return n, true // flate BestSpeed(1)..BestCompression(9)
}

func gzipLevel(n int) int {
	n = clampLevel(n)
	if n == 0 {
		return gzip.DefaultCompression
	}
	return n
}

// bzip2Level passes the 0..9 value straight through — dsnet/compress treats 0
// as its default and validates the 1..9 range itself.
func bzip2Level(n int) int { return clampLevel(n) }

func zstdLevel(n int) zstd.EncoderLevel {
	switch n = clampLevel(n); {
	case n == 0:
		return zstd.SpeedDefault
	case n <= 3:
		return zstd.SpeedFastest
	case n <= 6:
		return zstd.SpeedDefault
	case n <= 8:
		return zstd.SpeedBetterCompression
	default:
		return zstd.SpeedBestCompression
	}
}
