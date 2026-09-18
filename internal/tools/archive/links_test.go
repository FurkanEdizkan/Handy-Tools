package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// linkTree builds proj/{a.txt, sub/b.txt, link -> a.txt, dirlink -> sub} and
// returns the proj directory.
func linkTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"proj/a.txt":     "alpha",
		"proj/sub/b.txt": "bravo",
	})
	proj := filepath.Join(dir, "proj")
	if err := os.Symlink("a.txt", filepath.Join(proj, "link")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	if err := os.Symlink("sub", filepath.Join(proj, "dirlink")); err != nil {
		t.Fatalf("symlink dir: %v", err)
	}
	return proj
}

// TestCompressTarStoresSymlinksAsLinks is a regression for packing a directory
// that contains symlinks: the walk saw the link via Lstat (so the tar header
// said "symlink, size 0") but then copied the target's bytes into it, failing
// with "archive/tar: write too long" — and a link to a directory failed with
// "is a directory". Links must be stored as links, never followed.
func TestCompressTarStoresSymlinksAsLinks(t *testing.T) {
	for _, format := range []Format{FormatTar, FormatTarGz, FormatTarZst, FormatTarXz} {
		t.Run(format.String(), func(t *testing.T) {
			proj := linkTree(t)
			out := filepath.Join(t.TempDir(), "out."+format.String())
			last := runCompress(t, CompressRequest{Sources: []string{proj}, Format: format, Output: out})
			if last.Err != nil {
				t.Fatalf("compress failed: %v", last.Err)
			}

			got := readArchive(t, out, format)
			if got["proj/a.txt"] != "alpha" || got["proj/sub/b.txt"] != "bravo" {
				t.Fatalf("regular files mismatch: %#v", got)
			}
			links := tarLinks(t, out, format)
			if links["proj/link"] != "a.txt" || links["proj/dirlink"] != "sub" {
				t.Fatalf("symlinks not stored as links: %#v", links)
			}
		})
	}
}

// TestCompressZipStoresSymlinkTargets checks the zip side of the same bug: the
// entry was flagged as a symlink but its body was the *target file's content*
// ("alpha"), so `unzip` recreated a link pointing at a path named "alpha".
func TestCompressZipStoresSymlinkTargets(t *testing.T) {
	proj := linkTree(t)
	out := filepath.Join(t.TempDir(), "out.zip")
	last := runCompress(t, CompressRequest{Sources: []string{proj}, Format: FormatZip, Output: out})
	if last.Err != nil {
		t.Fatalf("compress failed: %v", last.Err)
	}

	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	links := map[string]string{}
	for _, f := range zr.File {
		if f.Mode()&os.ModeSymlink == 0 {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		links[f.Name] = string(b)
	}
	if links["proj/link"] != "a.txt" || links["proj/dirlink"] != "sub" {
		t.Fatalf("zip symlink bodies must be the link targets: %#v", links)
	}
}

// TestCompressFollowsSymlinkedSourceDir: a *source* the user named explicitly
// that is itself a symlink to a directory is packed by its contents, under the
// name the user gave — filepath.Walk alone would stop at the link.
func TestCompressFollowsSymlinkedSourceDir(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"real/a.txt": "alpha"})
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(filepath.Join(dir, "real"), alias); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	out := filepath.Join(dir, "out.tar")
	last := runCompress(t, CompressRequest{Sources: []string{alias}, Format: FormatTar, Output: out})
	if last.Err != nil {
		t.Fatalf("compress failed: %v", last.Err)
	}
	if got := readArchive(t, out, FormatTar); got["alias/a.txt"] != "alpha" {
		t.Fatalf("symlinked source dir not packed by content: %#v", got)
	}
}

// tarLinks returns name -> Linkname for every symlink entry in a tar archive.
func tarLinks(t *testing.T, path string, format Format) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	var r io.Reader = f
	switch format {
	case FormatTarGz:
		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		r = gz
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
	links := map[string]string{}
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return links
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		if h.Typeflag == tar.TypeSymlink {
			links[h.Name] = h.Linkname
		}
	}
}

// TestExtractTarZstReleasesDecoderOnEarlyExit: the zstd decoder decodes blocks
// on background goroutines, which only Close() stops. When extraction bails
// out early (here: a path-traversal entry rejected on the first header) the
// rest of the stream is never read, so an unclosed decoder leaks goroutines
// blocked on its output channel — for the lifetime of htoolsd / the GUI.
func TestExtractTarZstReleasesDecoderOnEarlyExit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "evil.tar.zst")
	payload := make([]byte, 4<<20)
	rand.New(rand.NewSource(1)).Read(payload) //nolint:gosec // deterministic filler
	writeTarWrapped(t, src, map[string]string{"../escape.txt": string(payload)},
		func(w io.Writer) (io.WriteCloser, error) { return zstd.NewWriter(w) })

	before := runtime.NumGoroutine()
	progress := collect(Extract(context.Background(), ExtractRequest{Source: src, Destination: filepath.Join(dir, "out")}))
	if last := progress[len(progress)-1]; last.Err == nil {
		t.Fatal("expected the traversal entry to be rejected")
	}

	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before {
		t.Fatalf("zstd decoder goroutines leaked: %d before, %d after", before, after)
	}
}

// TestCompressSevenZPassword: every surface documents --password as "7z only"
// (CLI help, MCP schema), but Compress rejected any password before looking
// at the format, so encrypted 7z creation was impossible. The archive must
// be created, and extracting it must require the password.
func TestCompressSevenZPassword(t *testing.T) {
	if !sysdep.Lookup("7z").Found {
		t.Skip("7z not on PATH")
	}
	dir := t.TempDir()
	src := writeFiles(t, dir, map[string]string{"f.txt": "secret body"})
	out := filepath.Join(dir, "out.7z")
	last := runCompress(t, CompressRequest{Sources: src, Format: FormatSevenZ, Output: out, Password: "pw"})
	if last.Err != nil {
		t.Fatalf("7z with password failed: %v", last.Err)
	}

	bad := collect(Extract(context.Background(), ExtractRequest{Source: out, Destination: filepath.Join(dir, "bad"), Password: "wrong"}))
	if bad[len(bad)-1].Err == nil {
		t.Fatal("extracting with the wrong password should fail — archive is not encrypted")
	}
	good := collect(Extract(context.Background(), ExtractRequest{Source: out, Destination: filepath.Join(dir, "good"), Password: "pw"}))
	if err := good[len(good)-1].Err; err != nil {
		t.Fatalf("extract with the right password: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "good", "f.txt")); err != nil || string(b) != "secret body" {
		t.Fatalf("round-trip body = %q, %v", b, err)
	}
}
