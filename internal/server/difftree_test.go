package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/difftree"
)

func TestDiffTreeHandlerRunReportsAddedAndRemoved(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if err := os.WriteFile(filepath.Join(a, "only-a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "only-b.txt"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &DiffTreeHandler{Opts: Options{AllowRoots: []string{a, b}}}
	var events []tools.Progress
	err := h.Run(context.Background(), DiffTreeParams{A: a, B: b, Mode: "mtime"}, func(p tools.Progress) error {
		events = append(events, p)
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// One event per diff (2) + terminal summary = 3.
	if len(events) < 3 {
		t.Fatalf("expected >=3 events, got %d", len(events))
	}
	if !events[len(events)-1].Completed {
		t.Fatal("last event must be Completed")
	}
}

func TestDiffTreeHandlerInspectReturnsDiffs(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if err := os.WriteFile(filepath.Join(a, "f.txt"), []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "f.txt"), []byte("v2-different-size"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &DiffTreeHandler{Opts: Options{AllowRoots: []string{a, b}}}
	diffs, err := h.Inspect(DiffTreeParams{A: a, B: b, Mode: "mtime"})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(diffs) != 1 || diffs[0].Status != difftree.StatusChanged {
		t.Fatalf("expected one changed diff, got %#v", diffs)
	}
}

func TestDiffTreeHandlerRejectsPathOutsideRoot(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	h := &DiffTreeHandler{Opts: Options{AllowRoots: []string{a}}}
	err := h.Run(context.Background(), DiffTreeParams{A: a, B: b, Mode: "mtime"}, func(p tools.Progress) error { return nil })
	if err == nil {
		t.Fatal("expected path-outside-root error for B")
	}
}
