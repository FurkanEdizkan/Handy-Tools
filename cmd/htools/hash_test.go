package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/config"
)

const sha256OfABC = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"

func writeBin(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, _ := os.Pipe()
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestCmdHashMissingFlags(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	cases := []struct {
		name string
		args []string
	}{
		{"no sources, no check", []string{"--algo", "sha256"}},
		{"bad algo", []string{"--algo", "galaxy", "/tmp/whatever"}},
		{"check + positional", []string{"--check", "SHA256SUMS", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := cmdHash(ctx, cfg, tc.args); code == 0 {
				t.Errorf("expected non-zero exit for %s", tc.name)
			}
		})
	}
}

func TestCmdHashSha256Output(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "in.bin")
	writeBin(t, p, "abc")

	out := captureStdout(t, func() {
		if code := cmdHash(context.Background(), config.Defaults(),
			[]string{"--algo", "sha256", "--quiet", p}); code != 0 {
			t.Fatalf("hash exit = %d, want 0", code)
		}
	})
	want := sha256OfABC + "  " + p + "\n"
	if out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestCmdHashCheckRoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeBin(t, a, "abc")
	writeBin(t, b, "abc")
	manifest := filepath.Join(dir, "SHA256SUMS")
	writeBin(t, manifest, sha256OfABC+"  a.txt\n"+sha256OfABC+"  b.txt\n")

	out := captureStdout(t, func() {
		if code := cmdHash(context.Background(), config.Defaults(),
			[]string{"--check", manifest, "--algo", "sha256", "--quiet"}); code != 0 {
			t.Fatalf("verify exit = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "a.txt: OK") || !strings.Contains(out, "b.txt: OK") {
		t.Errorf("verify output missing OK lines:\n%s", out)
	}
}

func TestCmdHashCheckCatchesMismatch(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeBin(t, a, "abc")
	manifest := filepath.Join(dir, "SHA256SUMS")
	// Wrong digest → row reports FAILED, command exits 1.
	writeBin(t, manifest, "0000000000000000000000000000000000000000000000000000000000000000  a.txt\n")

	out := captureStdout(t, func() {
		if code := cmdHash(context.Background(), config.Defaults(),
			[]string{"--check", manifest, "--algo", "sha256", "--quiet"}); code != 1 {
			t.Fatalf("verify mismatch exit = %d, want 1", code)
		}
	})
	if !strings.Contains(out, "a.txt: FAILED") {
		t.Errorf("verify output missing FAILED:\n%s", out)
	}
}

func TestCmdHashJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "in.bin")
	writeBin(t, p, "abc")

	out := captureStdout(t, func() {
		_ = cmdHash(context.Background(), config.Defaults(),
			[]string{"--algo", "sha256", "--json", "--quiet", p})
	})
	if !strings.Contains(out, `"digest":"`+sha256OfABC+`"`) {
		t.Errorf("--json output missing expected digest field:\n%s", out)
	}
}

// TestCmdHashCheckVerifiesItsOwnOutput: `htools hash d/a.txt > d/SUMS` then
// `htools hash --check d/SUMS` must pass — entries resolve against the cwd
// like `sha256sum -c`. They used to resolve only against the manifest's dir
// (d/d/a.txt), so htools couldn't verify its own manifests.
func TestCmdHashCheckVerifiesItsOwnOutput(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.Mkdir("d", 0o755); err != nil {
		t.Fatal(err)
	}
	writeBin(t, filepath.Join("d", "a.txt"), "abc")
	sums := captureStdout(t, func() {
		if code := cmdHash(context.Background(), config.Defaults(), []string{"--quiet", "d/a.txt"}); code != 0 {
			t.Fatalf("hash exit = %d", code)
		}
	})
	writeBin(t, filepath.Join("d", "SUMS"), sums)
	out := captureStdout(t, func() {
		if code := cmdHash(context.Background(), config.Defaults(), []string{"--quiet", "--check", "d/SUMS"}); code != 0 {
			t.Errorf("--check of its own manifest exit = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "d/a.txt: OK") {
		t.Errorf("verify output:\n%s", out)
	}
}

// TestCmdHashJSONReportsFailures: with --json, stdout is JSON lines only and
// ends with a terminal object whose failures list the files that couldn't be
// hashed (they used to go to stderr as text, with no terminal line at all).
func TestCmdHashJSONReportsFailures(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeBin(t, a, "abc")
	ghost := filepath.Join(dir, "ghost.txt")
	var code int
	out := captureStdout(t, func() {
		code = cmdHash(context.Background(), config.Defaults(), []string{"--json", a, ghost})
	})
	if code != 1 {
		t.Errorf("partial failure exit = %d, want 1 (like sha256sum)", code)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var last map[string]any
	for _, line := range lines {
		last = nil
		if err := json.Unmarshal([]byte(line), &last); err != nil {
			t.Fatalf("non-JSON line on stdout: %q", line)
		}
	}
	failures, _ := last["failures"].([]any)
	if last["completed"] != true || len(failures) != 1 {
		t.Fatalf("terminal line = %v, want completed with 1 failure", last)
	}
}
