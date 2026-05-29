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
	"github.com/ulikunitz/xz"

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
	case FormatTarXz:
		xr, err := xz.NewReader(f)
		if err != nil {
			t.Fatalf("xz reader: %v", err)
		}
		r = xr
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
		{"tar.xz", FormatTarXz, ".tar.xz"},
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

// TestCompressMixedSourceScenario verifies the multi-source preflight for
// pack: InspectCompress flags the missing path and the chmod-blocked
// destination without trying to write anything. Pack is a single-archive
// operation (not a per-file batch) so it doesn't have a Failures slice in
// the streaming sense — its preflight is the Inspection.Issues slice
// instead. Mirrors the mixed-batch scenario tests in
// internal/tools/{hash,rename,image,stripmeta}.
func TestCompressMixedSourceScenario(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — file modes are bypassed")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	if err := os.WriteFile(good, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "ghost.txt")
	// Read-only output dir so CheckOutputDirWritable surfaces the issue.
	outDir := filepath.Join(dir, "out-ro")
	if err := os.Mkdir(outDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(outDir, 0o755) })

	ins, terr := InspectCompress(CompressRequest{
		Sources: []string{good, missing},
		Format:  FormatZip,
		Output:  filepath.Join(outDir, "out.zip"),
	})
	if terr != nil {
		t.Fatalf("InspectCompress prelude error: %v", terr)
	}
	if len(ins.Issues) < 2 {
		t.Fatalf("want at least 2 issues (missing source + unwritable output), got %d: %+v", len(ins.Issues), ins.Issues)
	}
	var sawMissing, sawUnwritable bool
	for _, iss := range ins.Issues {
		if iss.Path == missing && iss.Code == "NOT_FOUND" {
			sawMissing = true
		}
		if iss.Path == outDir && iss.Code == "PERMISSION_DENIED" {
			sawUnwritable = true
		}
	}
	if !sawMissing {
		t.Errorf("missing source not flagged: %+v", ins.Issues)
	}
	if !sawUnwritable {
		t.Errorf("unwritable output dir not flagged: %+v", ins.Issues)
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
	// Overwrite=true skips the collision-avoidance rename, so the dir-at-output
	// scenario still drives Rename to a hard failure (which is what this test
	// is verifying — .partial cleanup on rename failure).
	last := runCompress(t, CompressRequest{
		Sources: []string{src}, Format: FormatZip, Output: finalOutput, Overwrite: true,
	})
	if last.Err == nil {
		t.Fatalf("expected failure when output path is a directory, got success")
	}
	if _, err := os.Stat(finalOutput + ".partial"); !os.IsNotExist(err) {
		t.Errorf("expected .partial to be cleaned up, got stat err = %v", err)
	}
}

// TestCompressDefaultDisambiguates: by default (Overwrite=false) a second pack
// to the same output filename writes to <stem>-1.<ext> instead of clobbering.
func TestCompressDefaultDisambiguates(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.zip")
	// Pre-create the target with a marker so we'd notice an overwrite.
	if err := os.WriteFile(out, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	last := runCompress(t, CompressRequest{
		Sources: []string{src}, Format: FormatZip, Output: out,
	})
	if last.Err != nil {
		t.Fatalf("compress failed: %+v", last.Err)
	}
	if got, _ := os.ReadFile(out); string(got) != "original" {
		t.Errorf("default Overwrite=false should leave existing file untouched; got %q", got)
	}
	disambiguated := filepath.Join(dir, "out-1.zip")
	if _, err := os.Stat(disambiguated); err != nil {
		t.Errorf("expected disambiguated archive at %s: %v", disambiguated, err)
	}
}

// TestCompressOverwriteReplaces: Overwrite=true replaces the existing file.
func TestCompressOverwriteReplaces(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.zip")
	if err := os.WriteFile(out, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	last := runCompress(t, CompressRequest{
		Sources: []string{src}, Format: FormatZip, Output: out, Overwrite: true,
	})
	if last.Err != nil {
		t.Fatalf("compress failed: %+v", last.Err)
	}
	// The file at `out` should now be a real zip, not "original".
	entries := readArchive(t, out, FormatZip)
	if entries["a.txt"] != "hello" {
		t.Errorf("overwritten archive contents wrong; got %v", entries)
	}
	if _, err := os.Stat(filepath.Join(dir, "out-1.zip")); !os.IsNotExist(err) {
		t.Errorf("Overwrite=true should not create out-1.zip")
	}
}

// TestCompressDisambiguationChain: when out.zip AND out-1.zip exist, default
// produces out-2.zip.
func TestCompressDisambiguationChain(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"out.zip", "out-1.zip"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	last := runCompress(t, CompressRequest{
		Sources: []string{src}, Format: FormatZip, Output: filepath.Join(dir, "out.zip"),
	})
	if last.Err != nil {
		t.Fatalf("compress failed: %+v", last.Err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out-2.zip")); err != nil {
		t.Errorf("expected out-2.zip: %v", err)
	}
}

// TestCompressCompoundExtensionDisambiguation: a.tar.gz becomes a-1.tar.gz,
// not a.tar-1.gz (verifies the compound-extension support).
func TestCompressCompoundExtensionDisambiguation(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "a.tar.gz")
	if err := os.WriteFile(out, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	last := runCompress(t, CompressRequest{
		Sources: []string{src}, Format: FormatTarGz, Output: out,
	})
	if last.Err != nil {
		t.Fatalf("compress failed: %+v", last.Err)
	}
	disambiguated := filepath.Join(dir, "a-1.tar.gz")
	if _, err := os.Stat(disambiguated); err != nil {
		t.Errorf("expected disambiguated archive at %s: %v", disambiguated, err)
	}
}
