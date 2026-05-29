package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func makeZip(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "in.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, body := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return path
}

func collect(ch <-chan tools.Progress) []tools.Progress {
	out := []tools.Progress{}
	for p := range ch {
		out = append(out, p)
	}
	return out
}

func TestInspectZipReportsEntryCount(t *testing.T) {
	dir := t.TempDir()
	src := makeZip(t, dir, map[string]string{"a.txt": "hello", "b.txt": "world"})

	ins, err := Inspect(context.Background(), src)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if ins.Format != FormatZip {
		t.Fatalf("format: got %s want zip", ins.Format)
	}
	if ins.EntryCount != 2 {
		t.Fatalf("entry count: got %d want 2", ins.EntryCount)
	}
	if ins.MultiPart {
		t.Fatalf("zip should not be multi-part")
	}
}

func TestExtractZipWritesAllEntries(t *testing.T) {
	dir := t.TempDir()
	src := makeZip(t, dir, map[string]string{"a.txt": "hello", "sub/b.txt": "world"})
	dst := filepath.Join(dir, "out")

	progress := collect(Extract(context.Background(), ExtractRequest{
		Source:      src,
		Destination: dst,
	}))
	last := progress[len(progress)-1]
	if last.Err != nil {
		t.Fatalf("extract: %v", last.Err)
	}

	body, err := os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(body) != "world" {
		t.Fatalf("body mismatch: got %q", string(body))
	}
}

// TestOutputDirFlagPerBinary locks in the unrar-vs-7z difference that broke
// every GUI Extract: unrar interprets `-o<path>` as "overwrite mode <path>"
// and exits "Unknown option: o<path>", while 7z uses `-o<path>` as the
// output-dir flag. unrar's actual output-path switch is `-op<path>`.
func TestOutputDirFlagPerBinary(t *testing.T) {
	if got := outputDirFlag("unrar", "/dest"); got != "-op/dest" {
		t.Errorf("unrar: got %q want %q", got, "-op/dest")
	}
	if got := outputDirFlag("7z", "/dest"); got != "-o/dest" {
		t.Errorf("7z: got %q want %q", got, "-o/dest")
	}
	if got := outputDirFlag("other", "/dest"); got != "-o/dest" {
		t.Errorf("default: got %q want %q", got, "-o/dest")
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	if _, err := safeJoin("/tmp/dest", "../escape.txt"); err == nil {
		t.Fatal("expected error for ../escape.txt")
	}
	if _, err := safeJoin("/tmp/dest", "/etc/passwd"); err == nil {
		t.Fatal("expected error for absolute path")
	}
	if _, err := safeJoin("/tmp/dest", "ok/inner.txt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindRarPartsDetectsMultiVolume(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"foo.part1.rar", "foo.part2.rar", "foo.part3.rar"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	parts, missing := findRarParts(filepath.Join(dir, "foo.part1.rar"))
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if len(missing) != 0 {
		t.Fatalf("expected no missing, got %v", missing)
	}
}

func TestFindRarPartsReportsMissing(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"foo.part1.rar", "foo.part3.rar"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	_, missing := findRarParts(filepath.Join(dir, "foo.part1.rar"))
	if len(missing) != 1 || missing[0] != "foo.part2.rar" {
		t.Fatalf("expected [foo.part2.rar], got %v", missing)
	}
}

func TestFindSevenZParts(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"bar.7z.001", "bar.7z.002"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	parts, missing := findSevenZParts(filepath.Join(dir, "bar.7z.001"))
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if len(missing) != 0 {
		t.Fatalf("expected no missing, got %v", missing)
	}
}

func writeTar(t *testing.T, path string, entries map[string]string, gz bool) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar: %v", err)
	}
	defer f.Close()
	var tw *tar.Writer
	if gz {
		gw := gzip.NewWriter(f)
		defer gw.Close()
		tw = tar.NewWriter(gw)
	} else {
		tw = tar.NewWriter(f)
	}
	defer tw.Close()
	for name, body := range entries {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("write body: %v", err)
		}
	}
}

func TestExtractTarWritesAllEntries(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.tar")
	writeTar(t, src, map[string]string{"a.txt": "hello", "sub/b.txt": "world"}, false)
	dst := filepath.Join(dir, "out")

	progress := collect(Extract(context.Background(), ExtractRequest{Source: src, Destination: dst}))
	if last := progress[len(progress)-1]; last.Err != nil {
		t.Fatalf("extract: %v", last.Err)
	}
	body, err := os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(body) != "world" {
		t.Fatalf("body mismatch: got %q", string(body))
	}
}

func TestExtractTarGzWritesAllEntries(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.tar.gz")
	writeTar(t, src, map[string]string{"hello.txt": "compressed body"}, true)
	dst := filepath.Join(dir, "out")

	progress := collect(Extract(context.Background(), ExtractRequest{Source: src, Destination: dst}))
	if last := progress[len(progress)-1]; last.Err != nil {
		t.Fatalf("extract: %v", last.Err)
	}
	body, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(body) != "compressed body" {
		t.Fatalf("body mismatch: got %q", string(body))
	}
}

// writeTarWrapped writes a tar archive through an arbitrary compression layer
// (xz, zstd, ...) so the extract-side decoders can be exercised end-to-end.
func writeTarWrapped(t *testing.T, path string, entries map[string]string, wrap func(io.Writer) (io.WriteCloser, error)) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer f.Close()
	w, err := wrap(f)
	if err != nil {
		t.Fatalf("wrap writer: %v", err)
	}
	tw := tar.NewWriter(w)
	for name, body := range entries {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("write body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close wrapper: %v", err)
	}
}

func TestExtractTarXzWritesAllEntries(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.tar.xz")
	writeTarWrapped(t, src, map[string]string{"hello.txt": "xz compressed body"},
		func(w io.Writer) (io.WriteCloser, error) { return xz.NewWriter(w) })
	dst := filepath.Join(dir, "out")

	progress := collect(Extract(context.Background(), ExtractRequest{Source: src, Destination: dst}))
	if last := progress[len(progress)-1]; last.Err != nil {
		t.Fatalf("extract: %v", last.Err)
	}
	body, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(body) != "xz compressed body" {
		t.Fatalf("body mismatch: got %q", string(body))
	}
}

// TestExtractTarZstWritesAllEntries is a regression for the tar.zst extract gap
// fixed alongside tar.xz: the format was detected and packable but missing from
// the Extract() switch, so extraction failed with "unsupported archive format".
func TestExtractTarZstWritesAllEntries(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.tar.zst")
	writeTarWrapped(t, src, map[string]string{"hello.txt": "zstd compressed body"},
		func(w io.Writer) (io.WriteCloser, error) { return zstd.NewWriter(w) })
	dst := filepath.Join(dir, "out")

	progress := collect(Extract(context.Background(), ExtractRequest{Source: src, Destination: dst}))
	if last := progress[len(progress)-1]; last.Err != nil {
		t.Fatalf("extract: %v", last.Err)
	}
	body, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(body) != "zstd compressed body" {
		t.Fatalf("body mismatch: got %q", string(body))
	}
}

// TestExtract7zRoundTripsViaBinary self-bootstraps a fixture using the system
// 7z binary, then exercises the binary-extraction code path. Skips locally
// when 7z isn't on PATH; CI installs p7zip-full so it actually runs there.
func TestExtract7zRoundTripsViaBinary(t *testing.T) {
	bin := ""
	for _, name := range []string{"7z", "7zz", "7za"} {
		if p, err := exec.LookPath(name); err == nil {
			bin = p
			break
		}
	}
	if bin == "" {
		t.Skip("7z not on PATH; skipping binary-path archive test")
	}

	dir := t.TempDir()
	plain := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(plain, []byte("seven zip body"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := filepath.Join(dir, "fixture.7z")
	cmd := exec.Command(bin, "a", "-bso0", "-bse0", src, plain)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create 7z fixture: %v\n%s", err, out)
	}

	dst := filepath.Join(dir, "out")
	progress := collect(Extract(context.Background(), ExtractRequest{Source: src, Destination: dst}))
	if last := progress[len(progress)-1]; last.Err != nil {
		t.Fatalf("extract: %v", last.Err)
	}
	body, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(body) != "seven zip body" {
		t.Fatalf("body mismatch: got %q", string(body))
	}
}
