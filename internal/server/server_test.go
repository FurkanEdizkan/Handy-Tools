package server

import (
	"path/filepath"
	"testing"
)

func TestCheckPathRejectsEmpty(t *testing.T) {
	if _, err := (Options{}).CheckPath(""); err == nil {
		t.Fatal("expected error on empty path")
	}
}

func TestCheckPathFailsClosedWithoutRoots(t *testing.T) {
	if _, err := (Options{}).CheckPath("/tmp/file"); err == nil {
		t.Fatal("expected error when no roots configured")
	}
}

func TestCheckPathAllowsUnderRoot(t *testing.T) {
	root := t.TempDir()
	abs, err := Options{AllowRoots: []string{root}}.CheckPath(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want, _ := filepath.Abs(filepath.Join(root, "a.txt"))
	if abs != want {
		t.Fatalf("path: got %q want %q", abs, want)
	}
}

func TestCheckPathRejectsTraversalOutsideRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := (Options{AllowRoots: []string{root}}).CheckPath("/etc/passwd"); err == nil {
		t.Fatal("expected error for path outside root")
	}
}
