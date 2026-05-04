package archive

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy/internal/tools"
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
