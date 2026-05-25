package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func TestHashHandlerRunSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &HashHandler{Opts: Options{AllowRoots: []string{dir}}}
	var events []tools.Progress
	err := h.Run(context.Background(), HashRunParams{
		Sources: []string{path},
		Algo:    "sha256",
	}, func(p tools.Progress) error {
		events = append(events, p)
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Expect at least one per-file event with the digest in Message + a terminal.
	if len(events) < 2 {
		t.Fatalf("expected >=2 progress events, got %d", len(events))
	}
	// SHA256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	wantDigest := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	found := false
	for _, e := range events {
		if e.Message != "" && len(e.Message) >= 64 && e.Message[:64] == wantDigest {
			found = true
		}
	}
	if !found {
		t.Fatalf("no event contained the expected sha256 digest; events: %#v", events)
	}
	if !events[len(events)-1].Completed {
		t.Fatal("last event must be Completed")
	}
}

func TestHashHandlerRunRejectsPathOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	h := &HashHandler{Opts: Options{AllowRoots: []string{dir}}}
	err := h.Run(context.Background(), HashRunParams{
		Sources: []string{"/etc/passwd"},
		Algo:    "sha256",
	}, func(p tools.Progress) error { return nil })
	if err == nil {
		t.Fatal("expected path-outside-root error")
	}
}

func TestHashHandlerVerifyMatchesManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifestBody := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824  a.txt\n"
	manifest := filepath.Join(dir, "SHA256SUMS")
	if err := os.WriteFile(manifest, []byte(manifestBody), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &HashHandler{Opts: Options{AllowRoots: []string{dir}}}
	res, err := h.Verify(context.Background(), HashVerifyParams{
		Manifest: manifest,
		Algo:     "sha256",
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if res.OK != 1 || res.Failed != 0 || res.Missing != 0 {
		t.Fatalf("counts: OK=%d Failed=%d Missing=%d", res.OK, res.Failed, res.Missing)
	}
}
