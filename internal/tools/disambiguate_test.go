package tools

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

func TestSplitOutputExt(t *testing.T) {
	cases := []struct {
		path, stem, ext string
	}{
		{"foo.png", "foo", ".png"},
		{"/a/b/foo.png", "/a/b/foo", ".png"},
		{"foo.tar.gz", "foo", ".tar.gz"},
		{"FOO.TAR.GZ", "FOO", ".TAR.GZ"},
		{"foo.tar.bz2", "foo", ".tar.bz2"},
		{"foo.tar.zst", "foo", ".tar.zst"},
		{"foo.tar.xz", "foo", ".tar.xz"},
		{"foo.tar", "foo", ".tar"},
		{"noext", "noext", ""},
		{"/a.b/c", "/a.b/c", ""},
	}
	for _, c := range cases {
		stem, ext := SplitOutputExt(c.path)
		if stem != c.stem || ext != c.ext {
			t.Errorf("SplitOutputExt(%q) = (%q, %q); want (%q, %q)",
				c.path, stem, ext, c.stem, c.ext)
		}
	}
}

func TestReserveUniquePath_FirstCandidateFree(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "foo.png")
	got, err := ReserveUniquePath(target)
	if err != nil {
		t.Fatalf("ReserveUniquePath: %v", err)
	}
	if got != target {
		t.Errorf("got %q, want %q", got, target)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("reserved file should exist: %v", err)
	}
}

func TestReserveUniquePath_RacingConcurrentWorkers(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "race.png")
	const n = 16
	results := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			got, err := ReserveUniquePath(target)
			if err != nil {
				t.Errorf("worker %d: %v", i, err)
				return
			}
			results[i] = got
		}(i)
	}
	wg.Wait()
	// Every result must be distinct.
	seen := map[string]int{}
	for _, r := range results {
		seen[r]++
	}
	if len(seen) != n {
		sort.Strings(results)
		t.Errorf("expected %d distinct paths, got %d: %v", n, len(seen), results)
	}
	// Every result must exist on disk.
	for _, r := range results {
		if _, err := os.Stat(r); err != nil {
			t.Errorf("reserved path %s missing: %v", r, err)
		}
	}
}

func TestNextUniquePath_FirstCandidateFree(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "foo.png")
	got, err := NextUniquePath(target)
	if err != nil {
		t.Fatalf("NextUniquePath: %v", err)
	}
	if got != target {
		t.Errorf("got %q, want %q", got, target)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Errorf("NextUniquePath must not touch the filesystem; stat err=%v", err)
	}
}

func TestNextUniquePath_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	must := func(p string) {
		t.Helper()
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(dir, "foo.zip"))
	must(filepath.Join(dir, "foo-1.zip"))
	got, err := NextUniquePath(filepath.Join(dir, "foo.zip"))
	if err != nil {
		t.Fatalf("NextUniquePath: %v", err)
	}
	want := filepath.Join(dir, "foo-2.zip")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNextUniquePath_CompoundExtension(t *testing.T) {
	dir := t.TempDir()
	must := func(p string) {
		t.Helper()
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(dir, "a.tar.gz"))
	got, err := NextUniquePath(filepath.Join(dir, "a.tar.gz"))
	if err != nil {
		t.Fatalf("NextUniquePath: %v", err)
	}
	want := filepath.Join(dir, "a-1.tar.gz")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReserveUniquePath_CompoundExtension(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.tar.gz")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReserveUniquePath(target)
	if err != nil {
		t.Fatalf("ReserveUniquePath: %v", err)
	}
	want := filepath.Join(dir, "a-1.tar.gz")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
