package sysdep

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLookupReturnsFoundForExistingBinary(t *testing.T) {
	dir := t.TempDir()
	name := "fake-7z"
	path := filepath.Join(dir, name)
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		t.Skip("PATH probing differs on windows")
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), mode); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	t.Setenv("PATH", dir)
	Reset()

	r := Lookup(name)
	if !r.Found {
		t.Fatalf("expected to find %s on synthetic PATH, got %+v", name, r)
	}
	if r.Path != path {
		t.Fatalf("path mismatch: got %s want %s", r.Path, path)
	}
}

func TestLookupReturnsNotFoundForMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	Reset()

	r := Lookup("definitely-not-a-real-tool-9999")
	if r.Found {
		t.Fatalf("expected missing tool to return Found=false, got %+v", r)
	}
}

func TestAllCoversKnownTools(t *testing.T) {
	results := All()
	if len(results) != len(Known) {
		t.Fatalf("All() returned %d results, want %d", len(results), len(Known))
	}
}
