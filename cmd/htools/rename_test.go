package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/config"
)

// seedDir drops touch-like fixtures into dir and returns dir for chaining.
func seedDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", n, err)
		}
	}
	return dir
}

func TestCmdRenameMissingFlags(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	dir := seedDir(t, "a.txt")
	cases := []struct {
		name string
		args []string
	}{
		{"no positional", []string{"--pattern", "a", "--replace", "b"}},
		{"two positionals", []string{"--pattern", "a", "--replace", "b", dir, dir}},
		{"no pattern", []string{"--replace", "b", dir}},
		{"no replace", []string{"--pattern", "a", dir}},
		{"bad collision", []string{"--pattern", "a", "--replace", "b", "--on-collision", "skiplog", dir}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := cmdRename(ctx, cfg, tc.args); code == 0 {
				t.Errorf("expected non-zero exit, got 0")
			}
		})
	}
}

func TestCmdRenameDryRun(t *testing.T) {
	dir := seedDir(t, "IMG_0001.JPG", "IMG_0002.JPG", "notes.txt")
	code := cmdRename(context.Background(), config.Defaults(),
		[]string{"--pattern", `IMG_(\d+)\.JPG`, "--replace", `photo-$1.jpg`, "--dry-run", dir},
	)
	if code != 0 {
		t.Errorf("dry-run exit = %d, want 0", code)
	}
	// Filesystem untouched — sources still present, no targets created.
	for _, want := range []string{"IMG_0001.JPG", "IMG_0002.JPG", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("dry-run mutated source %s: %v", want, err)
		}
	}
	for _, gone := range []string{"photo-0001.jpg", "photo-0002.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("dry-run created %s (err=%v); expected not-exist", gone, err)
		}
	}
}

func TestCmdRenameEndToEnd(t *testing.T) {
	dir := seedDir(t, "IMG_0001.JPG", "IMG_0002.JPG", "notes.txt")
	code := cmdRename(context.Background(), config.Defaults(),
		[]string{"--pattern", `IMG_(\d+)\.JPG`, "--replace", `photo-$1.jpg`, "--quiet", dir},
	)
	if code != 0 {
		t.Fatalf("rename exit = %d, want 0", code)
	}
	for _, want := range []string{"photo-0001.jpg", "photo-0002.jpg", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
	}
	for _, gone := range []string{"IMG_0001.JPG", "IMG_0002.JPG"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("source %s still present after rename (err=%v)", gone, err)
		}
	}
}

func TestParseCollisionTable(t *testing.T) {
	pairs := map[string]bool{
		"error": true, "ERROR": true, "": true,
		"skip": true, "Skip": true,
		"suffix": true, "SUFFIX": true,
		"galaxy": false, "ovrwrite": false,
	}
	for s, ok := range pairs {
		_, got := parseCollision(s)
		if got != ok {
			t.Errorf("parseCollision(%q) = %v, want %v", s, got, ok)
		}
	}
}

// TestCmdRenameEmptyReplace: an explicitly empty --replace (a separate empty
// argument, or `--replace=`) deletes the matched text. Both used to be rejected as "--replace is required", so there
// was no way to strip a substring — even though the error told you to type it.
func TestCmdRenameEmptyReplace(t *testing.T) {
	for _, args := range [][]string{
		{"--pattern", "_copy", "--replace", ""},
		{"--pattern", "_copy", "--replace="},
	} {
		dir := seedDir(t, "a_copy.txt")
		if code := cmdRename(context.Background(), config.Defaults(), append(args, "--quiet", dir)); code != 0 {
			t.Fatalf("%v: exit = %d, want 0", args, code)
		}
		if _, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil {
			t.Errorf("%v: a_copy.txt not renamed to a.txt: %v", args, err)
		}
	}
}

// TestCmdRenameDryRunJSON: with --json, stdout must carry JSON lines only.
func TestCmdRenameDryRunJSON(t *testing.T) {
	dir := seedDir(t, "IMG_0001.JPG", "notes.txt")
	out := captureStdout(t, func() {
		if code := cmdRename(context.Background(), config.Defaults(),
			[]string{"--pattern", `IMG_(\d+)\.JPG`, "--replace", `photo-$1.jpg`, "--dry-run", "--json", dir}); code != 0 {
			t.Errorf("exit = %d, want 0", code)
		}
	})
	var kinds []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("non-JSON line on stdout: %q (%v)", line, err)
		}
		kinds = append(kinds, obj["kind"].(string))
	}
	if len(kinds) != 3 || kinds[2] != "summary" {
		t.Fatalf("kinds = %v, want two plan rows then a summary", kinds)
	}
}
