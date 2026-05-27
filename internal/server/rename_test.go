package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func TestRenameHandlerInspectReturnsPlan(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &RenameHandler{Opts: Options{AllowRoots: []string{dir}}}
	ins, err := h.Inspect(RenameParams{
		Sources: []string{src},
		Pattern: `^old\.txt$`,
		Replace: "new.txt",
	})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(ins.Plans) != 1 || ins.Plans[0].To != filepath.Join(dir, "new.txt") {
		t.Fatalf("unexpected plan: %#v", ins.Plans)
	}
}

func TestRenameHandlerRunRenamesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &RenameHandler{Opts: Options{AllowRoots: []string{dir}}}
	err := h.Run(context.Background(), RenameParams{
		Sources: []string{src},
		Pattern: `^old\.txt$`,
		Replace: "new.txt",
	}, func(p tools.Progress) error { return nil })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Fatalf("renamed file not present: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("old file should be gone, got err=%v", err)
	}
}

func TestRenameHandlerRejectsPathOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	h := &RenameHandler{Opts: Options{AllowRoots: []string{dir}}}
	err := h.Run(context.Background(), RenameParams{
		Sources: []string{"/tmp/outside.txt"},
		Pattern: `.*`,
		Replace: "x",
	}, func(p tools.Progress) error { return nil })
	if err == nil {
		t.Fatal("expected path-outside-root error")
	}
}
