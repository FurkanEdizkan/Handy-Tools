package main

import (
	"context"
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
