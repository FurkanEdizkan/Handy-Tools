package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// writeFiles drops name->content files under dir and returns their paths.
func writeFiles(t *testing.T, dir string, files map[string]string) []string {
	t.Helper()
	var paths []string
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		paths = append(paths, p)
	}
	return paths
}

// runCompress runs Compress to completion and returns the terminal progress.
func runCompress(t *testing.T, req CompressRequest) tools.Progress {
	t.Helper()
	progs := collect(Compress(context.Background(), req))
	if len(progs) == 0 {
		t.Fatal("Compress emitted no progress")
	}
	last := progs[len(progs)-1]
	if !last.Completed {
		t.Fatalf("Compress did not emit a terminal progress; last = %+v", last)
	}
	return last
}

// readArchive decodes an archive created by Compress into a name->content map.
func readArchive(t *testing.T, path string, format Format) map[string]string {
	t.Helper()
	got := map[string]string{}

	if format == FormatZip {
		zr, err := zip.OpenReader(path)
		if err != nil {
			t.Fatalf("open zip: %v", err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open zip entry %s: %v", f.Name, err)
			}
			b, _ := io.ReadAll(rc)
			rc.Close()
			got[f.Name] = string(b)
		}
		return got
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()

	var r io.Reader = f
	switch format {
	case FormatTarGz:
		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		defer gz.Close()
		r = gz
	case FormatTarBz2:
		r = bzip2.NewReader(f)
	case FormatTarZst:
		zr, err := zstd.NewReader(f)
		if err != nil {
			t.Fatalf("zstd reader: %v", err)
		}
		defer zr.Close()
		r = zr
	}

	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		b, _ := io.ReadAll(tr)
		got[h.Name] = string(b)
	}
	return got
}

func TestCompressZipRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := writeFiles(t, dir, map[string]string{
		"hello.txt": "hello world",
		"data.csv":  "a,b,c",
	})
	out := filepath.Join(dir, "out.zip")

	if last := runCompress(t, CompressRequest{Sources: src, Format: FormatZip, Output: out}); last.Err != nil {
		t.Fatalf("compress zip failed: %v", last.Err)
	}
	got := readArchive(t, out, FormatZip)
	if got["hello.txt"] != "hello world" || got["data.csv"] != "a,b,c" {
		t.Fatalf("zip round-trip mismatch: %#v", got)
	}
}

func TestCompressTarVariants(t *testing.T) {
	cases := []struct {
		name   string
		format Format
		ext    string
	}{
		{"tar", FormatTar, ".tar"},
		{"tar.gz", FormatTarGz, ".tar.gz"},
		{"tar.bz2", FormatTarBz2, ".tar.bz2"},
		{"tar.zst", FormatTarZst, ".tar.zst"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := writeFiles(t, dir, map[string]string{
				"notes.md": "# title\nbody",
				"x.txt":    "xyz",
			})
			out := filepath.Join(dir, "archive"+tc.ext)

			last := runCompress(t, CompressRequest{
				Sources: src, Format: tc.format, Output: out, CompressionLevel: 6,
			})
			if last.Err != nil {
				t.Fatalf("compress %s failed: %v", tc.name, last.Err)
			}
			got := readArchive(t, out, tc.format)
			if got["notes.md"] != "# title\nbody" || got["x.txt"] != "xyz" {
				t.Fatalf("%s round-trip mismatch: %#v", tc.name, got)
			}
		})
	}
}

func TestCompressRecursesDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"proj/a.txt":     "alpha",
		"proj/sub/b.txt": "bravo",
	})
	out := filepath.Join(dir, "proj.tar")

	last := runCompress(t, CompressRequest{
		Sources: []string{filepath.Join(dir, "proj")}, Format: FormatTar, Output: out,
	})
	if last.Err != nil {
		t.Fatalf("compress dir failed: %v", last.Err)
	}
	got := readArchive(t, out, FormatTar)
	if got["proj/a.txt"] != "alpha" || got["proj/sub/b.txt"] != "bravo" {
		t.Fatalf("directory recursion mismatch: %#v", got)
	}
}

func TestCompressInfersFormatFromOutputExtension(t *testing.T) {
	dir := t.TempDir()
	src := writeFiles(t, dir, map[string]string{"f.txt": "data"})
	out := filepath.Join(dir, "inferred.tar.gz")

	// Format left as FormatUnknown — Compress should infer tar.gz from ".tar.gz".
	last := runCompress(t, CompressRequest{Sources: src, Output: out})
	if last.Err != nil {
		t.Fatalf("compress with inferred format failed: %v", last.Err)
	}
	if got := readArchive(t, out, FormatTarGz); got["f.txt"] != "data" {
		t.Fatalf("inferred-format round-trip mismatch: %#v", got)
	}
}

func TestCompressRarUnsupported(t *testing.T) {
	dir := t.TempDir()
	src := writeFiles(t, dir, map[string]string{"f.txt": "data"})

	last := runCompress(t, CompressRequest{
		Sources: src, Format: FormatRar, Output: filepath.Join(dir, "out.rar"),
	})
	if last.Err == nil || last.Err.Code != tools.CodeUnsupportedInput {
		t.Fatalf("expected UNSUPPORTED_INPUT for RAR, got %+v", last.Err)
	}
}

func TestCompressPasswordUnsupported(t *testing.T) {
	dir := t.TempDir()
	src := writeFiles(t, dir, map[string]string{"f.txt": "data"})

	last := runCompress(t, CompressRequest{
		Sources: src, Format: FormatZip, Output: filepath.Join(dir, "out.zip"), Password: "secret",
	})
	if last.Err == nil || last.Err.Code != tools.CodeUnsupportedInput {
		t.Fatalf("expected UNSUPPORTED_INPUT when a password is set, got %+v", last.Err)
	}
}

func TestCompressEmptySourcesRejected(t *testing.T) {
	last := runCompress(t, CompressRequest{
		Sources: nil, Format: FormatZip, Output: filepath.Join(t.TempDir(), "out.zip"),
	})
	if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST for empty sources, got %+v", last.Err)
	}
}

// TestCompressNoPartialOnSuccess verifies the .partial staging file is
// cleaned up after a successful pack — only the final output should remain
// on disk, never a sibling .partial.
func TestCompressNoPartialOnSuccess(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.zip")
	last := runCompress(t, CompressRequest{
		Sources: []string{src}, Format: FormatZip, Output: out,
	})
	if last.Err != nil {
		t.Fatalf("compress failed: %+v", last.Err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("expected final output to exist: %v", err)
	}
	if _, err := os.Stat(out + ".partial"); !os.IsNotExist(err) {
		t.Errorf("expected no .partial leftover, got stat err = %v", err)
	}
}

// TestCompressRemovesPartialOnFailure forces a finalize-rename failure
// (output path is a directory) and confirms the .partial staging file is
// removed rather than left as an orphan.
func TestCompressRemovesPartialOnFailure(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make the final output path a directory so os.Rename(staged, final)
	// must fail. The .partial file is the rename's source; whatever happens
	// in the renamer, the cleanup branch should still remove it.
	finalOutput := filepath.Join(dir, "out.zip")
	if err := os.Mkdir(finalOutput, 0o755); err != nil {
		t.Fatal(err)
	}
	last := runCompress(t, CompressRequest{
		Sources: []string{src}, Format: FormatZip, Output: finalOutput,
	})
	if last.Err == nil {
		t.Fatalf("expected failure when output path is a directory, got success")
	}
	if _, err := os.Stat(finalOutput + ".partial"); !os.IsNotExist(err) {
		t.Errorf("expected .partial to be cleaned up, got stat err = %v", err)
	}
}
