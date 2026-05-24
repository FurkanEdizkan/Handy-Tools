package main

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/config"
)

// writeTinyZip creates a zip at dst containing one entry per (name, body)
// pair. Each body is written verbatim — caller provides line breaks if they
// want multi-line content for the grep tests to find.
func writeTinyZip(t *testing.T, dst string, entries map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry %s: %v", name, err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := os.WriteFile(dst, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func TestCmdInspectMissingArgs(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	cases := []struct {
		name string
		args []string
	}{
		{"no source", []string{}},
		{"too many sources", []string{"a.zip", "b.zip"}},
		{"bad grep regex", []string{"--grep", "[", "a.zip"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := cmdInspect(ctx, cfg, tc.args); code == 0 {
				t.Errorf("expected non-zero exit, got 0")
			}
		})
	}
}

func TestCmdInspectMetadata(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "tiny.zip")
	writeTinyZip(t, zipPath, map[string]string{
		"a.txt":     "hello\n",
		"sub/b.txt": "world\n",
	})

	if code := cmdInspect(context.Background(), config.Defaults(),
		[]string{"--json", zipPath},
	); code != 0 {
		t.Errorf("inspect metadata exit = %d, want 0", code)
	}
}

func TestCmdInspectGrepMatches(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "code.zip")
	writeTinyZip(t, zipPath, map[string]string{
		"main.go":  "package main\n// TODO: fix this later\nfunc main() {}\n",
		"util.go":  "package util\nfunc Add() {}\n",
		"notes.md": "Things to do: TODO check the migration\n",
	})

	// Capture os.Stdout so we can verify the match output. The function uses
	// fmt.Fprintf(os.Stdout, …) so redirecting os.Stdout is the cleanest way.
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	code := cmdInspect(context.Background(), config.Defaults(),
		[]string{"--grep", "TODO", "--quiet", zipPath},
	)
	w.Close()

	got, _ := io.ReadAll(r)
	if code != 0 {
		t.Errorf("grep exit = %d, want 0 (matches found), output:\n%s", code, got)
	}
	out := string(got)
	if !regexp.MustCompile(`main\.go:2:`).MatchString(out) {
		t.Errorf("missing main.go:2 in output:\n%s", out)
	}
	if !regexp.MustCompile(`notes\.md:1:`).MatchString(out) {
		t.Errorf("missing notes.md:1 in output:\n%s", out)
	}
}

func TestCmdInspectGrepNoMatches(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "nada.zip")
	writeTinyZip(t, zipPath, map[string]string{
		"a.txt": "hello world\n",
	})

	code := cmdInspect(context.Background(), config.Defaults(),
		[]string{"--grep", "TODO", "--quiet", zipPath},
	)
	// Convention (like grep(1)): exit 1 when nothing matched.
	if code != 1 {
		t.Errorf("no-match exit = %d, want 1", code)
	}
}

func TestCmdInspectGrepSkipsLargeEntries(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "big.zip")
	// 4 KiB of content with TODO at the end. With --max-bytes 1024 the entry
	// is skipped before its content reaches the matcher.
	big := bytes.Repeat([]byte("x"), 4096) // larger than --max-bytes
	writeTinyZip(t, zipPath, map[string]string{
		"big.txt": string(big) + "\nTODO\n",
	})

	code := cmdInspect(context.Background(), config.Defaults(),
		[]string{"--grep", "TODO", "--max-bytes", "1024", "--quiet", zipPath},
	)
	if code != 1 {
		t.Errorf("oversized-skip exit = %d, want 1 (no matches because entry skipped)", code)
	}
}
