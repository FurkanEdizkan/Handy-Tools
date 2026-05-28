package difftree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestFileDiff_IdenticalFilesProduceNoLines(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	writeFile(t, a, "hello\nworld\n")
	writeFile(t, b, "hello\nworld\n")
	res, err := FileDiff(a, b)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	if !res.Identical {
		t.Errorf("expected Identical=true, got %+v", res)
	}
	if len(res.Lines) != 0 {
		t.Errorf("expected no lines, got %d", len(res.Lines))
	}
}

func TestFileDiff_OneLineChangeReportsAddRemove(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	writeFile(t, a, "alpha\nbeta\ngamma\n")
	writeFile(t, b, "alpha\nBETA\ngamma\n")
	res, err := FileDiff(a, b)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	if res.Identical || res.Binary {
		t.Fatalf("unexpected flags: %+v", res)
	}
	var sawAdd, sawRemove, sawHunk bool
	for _, l := range res.Lines {
		switch l.Kind {
		case FileDiffAdd:
			if l.Text == "BETA" {
				sawAdd = true
			}
		case FileDiffRemove:
			if l.Text == "beta" {
				sawRemove = true
			}
		case FileDiffHunk:
			sawHunk = true
		}
	}
	if !sawAdd || !sawRemove {
		t.Errorf("expected add+remove for changed line, got %+v", res.Lines)
	}
	if !sawHunk {
		t.Errorf("expected a hunk separator, got %+v", res.Lines)
	}
}

func TestFileDiff_DetectsBinary(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	writeFile(t, a, "text on A\n")
	if err := os.WriteFile(b, []byte{'P', 'N', 'G', 0x00, 'b', 'i', 'n'}, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := FileDiff(a, b)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	if !res.Binary {
		t.Errorf("expected Binary=true, got %+v", res)
	}
	if len(res.Lines) != 0 {
		t.Errorf("binary diff should not include lines")
	}
}

func TestFileDiff_TruncatesLargeFiles(t *testing.T) {
	dir := t.TempDir()
	orig := FileDiffLimit
	FileDiffLimit = 64
	defer func() { FileDiffLimit = orig }()

	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	writeFile(t, a, strings.Repeat("aaaaaaaa\n", 20)) // 180 bytes > 64
	writeFile(t, b, strings.Repeat("bbbbbbbb\n", 20))
	res, err := FileDiff(a, b)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	if !res.Truncated {
		t.Errorf("expected Truncated=true, got %+v", res)
	}
	if len(res.Lines) == 0 {
		t.Errorf("truncated diff should still report the changed prefix")
	}
}

func TestFileDiff_MissingFileSurfacesNotFound(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "exists")
	writeFile(t, a, "x\n")
	_, err := FileDiff(a, filepath.Join(dir, "absent"))
	if err == nil {
		t.Fatal("expected error for missing B path")
	}
	if err.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %q (%s)", err.Code, err.Message)
	}
}
