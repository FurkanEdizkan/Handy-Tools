// Package archive inspects, extracts and creates archives. Pure Go is used
// for zip / tar / gz / bz2 / zstd. RAR and 7z (including multi-part volumes)
// are delegated to the system "unrar" and "7z" binaries when present.
package archive

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// Format enumerates the archive types Handy Tools understands.
type Format int

const (
	FormatUnknown Format = iota
	FormatZip
	FormatTar
	FormatTarGz
	FormatTarBz2
	FormatTarZst
	FormatRar
	FormatSevenZ
)

func (f Format) String() string {
	return [...]string{"unknown", "zip", "tar", "tar.gz", "tar.bz2", "tar.zst", "rar", "7z"}[f]
}

// Inspection is the result of Inspect.
type Inspection struct {
	Format          Format
	MultiPart       bool
	DetectedParts   []string // absolute paths in volume order
	MissingParts    []string // expected filenames not found
	UncompressedSz  int64    // best effort; -1 if unknown
	EntryCount      int      // best effort; -1 if unknown
	RequiresPwd     bool
	RequiresBinary  string // "" if pure Go can handle it
	BinaryAvailable bool
}

// Inspect describes an archive without extracting it. For multi-part archives
// it walks the directory looking for sibling parts. Use the result to confirm
// with the user before kicking off Extract.
func Inspect(_ context.Context, source string) (*Inspection, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("inspect: %s is a directory, expected a file", source)
	}

	ins := &Inspection{
		Format:         detectFormat(source),
		UncompressedSz: -1,
		EntryCount:     -1,
		DetectedParts:  []string{source},
	}
	switch ins.Format {
	case FormatRar:
		ins.RequiresBinary = "unrar"
		ins.BinaryAvailable = sysdep.Lookup("unrar").Found
		parts, missing := findRarParts(source)
		if len(parts) > 1 {
			ins.MultiPart = true
			ins.DetectedParts = parts
			ins.MissingParts = missing
		}
	case FormatSevenZ:
		ins.RequiresBinary = "7z"
		ins.BinaryAvailable = sysdep.Lookup("7z").Found
		parts, missing := findSevenZParts(source)
		if len(parts) > 1 {
			ins.MultiPart = true
			ins.DetectedParts = parts
			ins.MissingParts = missing
		}
	case FormatZip:
		count, size, err := summarizeZip(source)
		if err == nil {
			ins.EntryCount = count
			ins.UncompressedSz = size
		}
	}
	return ins, nil
}

// ExtractRequest describes an extraction job.
type ExtractRequest struct {
	Source        string
	Parts         []string // optional; if empty, inferred via Inspect
	Destination   string
	Password      string
	Overwrite     bool
	AutoMultiPart bool // when true, multi-part archives proceed without confirmation
}

// Extract performs the extraction. Multi-part archives where AutoMultiPart is
// false will return an ABORTED error if the caller didn't pre-confirm by
// supplying the Parts slice — the TUI calls Inspect first, asks the user, then
// passes Parts back in.
func Extract(ctx context.Context, req ExtractRequest) <-chan tools.Progress {
	ch := make(chan tools.Progress, 8)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "archive"
			p.Action = "extract"
			p.StartedAt = started
			if p.CurrentItem == "" {
				p.CurrentItem = filepath.Base(req.Source)
			}
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		if err := os.MkdirAll(req.Destination, 0o755); err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: "create destination", Detail: err.Error()},
			})
			return
		}

		ins, err := Inspect(ctx, req.Source)
		if err != nil {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: "inspect failed", Detail: err.Error()},
			})
			return
		}

		if ins.MultiPart && len(ins.MissingParts) > 0 {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code:    tools.CodeBadRequest,
				Message: "multi-part archive is missing volumes",
				Detail:  strings.Join(ins.MissingParts, ", "),
			}})
			return
		}
		if ins.MultiPart && !req.AutoMultiPart && len(req.Parts) == 0 {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code:    tools.CodeAborted,
				Message: "multi-part archive requires confirmation",
				Detail:  fmt.Sprintf("%d volumes detected; pass them in req.Parts or set AutoMultiPart", len(ins.DetectedParts)),
			}})
			return
		}

		emit(tools.Progress{Level: tools.SeverityInfo,
			Message: fmt.Sprintf("extracting %s archive into %s", ins.Format, req.Destination)})

		switch ins.Format {
		case FormatZip:
			err = extractZip(ctx, req, emit)
		case FormatTar:
			err = extractTar(ctx, req, emit, plainOpener)
		case FormatTarGz:
			err = extractTar(ctx, req, emit, gzipOpener)
		case FormatTarBz2:
			err = extractTar(ctx, req, emit, bzip2Opener)
		case FormatRar:
			err = extractViaBinary(ctx, "unrar", []string{"x", "-y"}, req.Source, req.Destination, req.Password, emit)
		case FormatSevenZ:
			err = extractViaBinary(ctx, "7z", []string{"x", "-y"}, req.Source, req.Destination, req.Password, emit)
		default:
			err = fmt.Errorf("unsupported archive format: %s", ins.Format)
		}
		if err != nil {
			var terr *tools.Error
			if errors.As(err, &terr) {
				emit(tools.Progress{Completed: true, Err: terr})
			} else {
				emit(tools.Progress{Completed: true, Err: &tools.Error{
					Code: tools.CodeIO, Message: "extract failed", Detail: err.Error(),
				}})
			}
			return
		}
		emit(tools.Progress{Completed: true, Fraction: 1, Level: tools.SeverityInfo,
			Message: "extraction complete"})
	}()
	return ch
}

// ---- format detection -------------------------------------------------------

var (
	rarPartRE       = regexp.MustCompile(`(?i)\.part(\d+)\.rar$`)
	rarOldPartRE    = regexp.MustCompile(`(?i)\.r(\d+)$`)
	sevenZVolumeRE  = regexp.MustCompile(`(?i)\.7z\.(\d+)$`)
	zipFirstHeaders = [][]byte{{0x50, 0x4b, 0x03, 0x04}, {0x50, 0x4b, 0x05, 0x06}}
)

func detectFormat(path string) Format {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return FormatZip
	case strings.HasSuffix(lower, ".tar"):
		return FormatTar
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return FormatTarGz
	case strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2"):
		return FormatTarBz2
	case strings.HasSuffix(lower, ".tar.zst") || strings.HasSuffix(lower, ".tzst"):
		return FormatTarZst
	case strings.HasSuffix(lower, ".rar") || rarOldPartRE.MatchString(lower):
		return FormatRar
	case strings.HasSuffix(lower, ".7z") || sevenZVolumeRE.MatchString(lower):
		return FormatSevenZ
	}
	return sniff(path)
}

func sniff(path string) Format {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return FormatUnknown
	}
	defer f.Close()
	head := make([]byte, 8)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	for _, sig := range zipFirstHeaders {
		if bytes.HasPrefix(head, sig) {
			return FormatZip
		}
	}
	if bytes.HasPrefix(head, []byte("Rar!")) {
		return FormatRar
	}
	if bytes.HasPrefix(head, []byte{0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c}) {
		return FormatSevenZ
	}
	return FormatUnknown
}

// findRarParts locates .partN.rar siblings (or .r00 / .r01 legacy style).
// It returns parts in volume order plus filenames that are expected but
// missing on disk.
func findRarParts(source string) (parts []string, missing []string) {
	dir := filepath.Dir(source)
	base := filepath.Base(source)
	if m := rarPartRE.FindStringSubmatch(base); m != nil {
		stem := base[:len(base)-len(m[0])]
		entries, err := os.ReadDir(dir)
		if err != nil {
			return []string{source}, nil
		}
		idxs := []int{}
		byIdx := map[int]string{}
		for _, e := range entries {
			n := e.Name()
			if !strings.HasPrefix(n, stem+".part") || !strings.HasSuffix(strings.ToLower(n), ".rar") {
				continue
			}
			if mm := rarPartRE.FindStringSubmatch(n); mm != nil {
				var i int
				_, _ = fmt.Sscanf(mm[1], "%d", &i)
				byIdx[i] = filepath.Join(dir, n)
				idxs = append(idxs, i)
			}
		}
		sort.Ints(idxs)
		for i, want := 0, 0; i < len(idxs); i++ {
			want++
			for want < idxs[i] {
				missing = append(missing, fmt.Sprintf("%s.part%d.rar", stem, want))
				want++
			}
		}
		for _, i := range idxs {
			parts = append(parts, byIdx[i])
		}
		return parts, missing
	}
	// Legacy .rNN scheme: <base>.rar, <base>.r00, <base>.r01...
	if strings.HasSuffix(strings.ToLower(base), ".rar") {
		stem := base[:len(base)-4]
		entries, _ := os.ReadDir(dir)
		extras := []string{}
		for _, e := range entries {
			n := e.Name()
			if rarOldPartRE.MatchString(n) && strings.HasPrefix(n, stem+".") {
				extras = append(extras, filepath.Join(dir, n))
			}
		}
		if len(extras) == 0 {
			return []string{source}, nil
		}
		sort.Strings(extras)
		return append([]string{source}, extras...), nil
	}
	return []string{source}, nil
}

func findSevenZParts(source string) (parts []string, missing []string) {
	dir := filepath.Dir(source)
	base := filepath.Base(source)
	m := sevenZVolumeRE.FindStringSubmatch(base)
	if m == nil {
		return []string{source}, nil
	}
	stem := base[:len(base)-len(m[0])]
	entries, _ := os.ReadDir(dir)
	idxs := []int{}
	byIdx := map[int]string{}
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, stem+".7z.") {
			continue
		}
		if mm := sevenZVolumeRE.FindStringSubmatch(n); mm != nil {
			var i int
			_, _ = fmt.Sscanf(mm[1], "%d", &i)
			byIdx[i] = filepath.Join(dir, n)
			idxs = append(idxs, i)
		}
	}
	sort.Ints(idxs)
	for i, want := 0, 1; i < len(idxs); i++ {
		for want < idxs[i] {
			missing = append(missing, fmt.Sprintf("%s.7z.%03d", stem, want))
			want++
		}
		want++
	}
	for _, i := range idxs {
		parts = append(parts, byIdx[i])
	}
	return parts, missing
}

// ---- zip / tar extraction ---------------------------------------------------

func summarizeZip(path string) (count int, total int64, err error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return 0, 0, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		count++
		total += int64(f.UncompressedSize64) //nolint:gosec
	}
	return count, total, nil
}

type emitFn func(tools.Progress)

func extractZip(ctx context.Context, req ExtractRequest, emit emitFn) error {
	zr, err := zip.OpenReader(req.Source)
	if err != nil {
		return err
	}
	defer zr.Close()

	var totalBytes int64
	for _, f := range zr.File {
		totalBytes += int64(f.UncompressedSize64) //nolint:gosec
	}
	var done int64

	for i, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		out, err := safeJoin(req.Destination, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(out, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if !req.Overwrite {
			if _, err := os.Stat(out); err == nil {
				return fmt.Errorf("output exists: %s", out)
			}
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		n, copyErr := io.Copy(w, rc) //nolint:gosec // size-bounded by archive entry
		rc.Close()
		w.Close()
		if copyErr != nil {
			return copyErr
		}
		done += n
		frac := 0.0
		if totalBytes > 0 {
			frac = float64(done) / float64(totalBytes)
		}
		emit(tools.Progress{
			Level: tools.SeverityInfo, Message: fmt.Sprintf("[%d/%d] %s", i+1, len(zr.File), f.Name),
			BytesDone: done, BytesTotal: totalBytes, Fraction: frac, CurrentItem: f.Name,
		})
	}
	return nil
}

type tarOpener func(io.Reader) (io.Reader, error)

func plainOpener(r io.Reader) (io.Reader, error) { return r, nil }
func gzipOpener(r io.Reader) (io.Reader, error)  { return gzip.NewReader(r) }
func bzip2Opener(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil }

func extractTar(ctx context.Context, req ExtractRequest, emit emitFn, open tarOpener) error {
	f, err := os.Open(req.Source) //nolint:gosec
	if err != nil {
		return err
	}
	defer f.Close()
	r, err := open(bufio.NewReader(f))
	if err != nil {
		return err
	}
	tr := tar.NewReader(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		out, err := safeJoin(req.Destination, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(out, os.FileMode(h.Mode)); err != nil { //nolint:gosec
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			if !req.Overwrite {
				if _, err := os.Stat(out); err == nil {
					return fmt.Errorf("output exists: %s", out)
				}
			}
			w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode)) //nolint:gosec
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil { //nolint:gosec
				w.Close()
				return err
			}
			w.Close()
			emit(tools.Progress{Level: tools.SeverityInfo, Message: h.Name, CurrentItem: h.Name})
		}
	}
}

// ---- shell-out (rar / 7z) ---------------------------------------------------

func extractViaBinary(ctx context.Context, name string, baseArgs []string, source, dest, password string, emit emitFn) error {
	r := sysdep.Lookup(name)
	if !r.Found {
		return &tools.Error{
			Code:    tools.CodeMissingBinary,
			Message: fmt.Sprintf("%s not found on PATH", name),
			Detail:  fmt.Sprintf("install hint: %s", r.Tool.InstallHint["linux"]),
		}
	}
	args := append([]string{}, baseArgs...)
	if password != "" {
		args = append(args, "-p"+password)
	}
	args = append(args, source, "-o"+dest)

	cmd := exec.CommandContext(ctx, r.UsedAlias, args...) //nolint:gosec
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		emit(tools.Progress{Level: tools.SeverityInfo, Message: scanner.Text()})
	}
	return cmd.Wait()
}

// safeJoin prevents Zip-Slip by ensuring the joined path stays under base.
func safeJoin(base, name string) (string, error) {
	clean := filepath.Clean(name)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "/../") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("unsafe entry path: %s", name)
	}
	return filepath.Join(base, clean), nil
}
