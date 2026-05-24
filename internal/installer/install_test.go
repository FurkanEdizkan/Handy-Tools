// Package installer holds tests for the repo-root install.sh shell installer.
//
// install.sh contains no Go code, so these tests shell out to it. They live in
// the Go tree purely so `go test ./...` (and therefore CI) exercises the
// installer's pure-shell logic that needs no network — currently the banner.
package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// installScript returns the absolute path to the repo-root install.sh.
func installScript(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed; cannot locate install.sh")
	}
	p := filepath.Join(filepath.Dir(thisFile), "..", "..", "install.sh")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("install.sh not found at %s: %v", p, err)
	}
	return p
}

// runHelp runs `install.sh --help` with the given extra environment and
// returns its combined output. `--help` reaches usage() -> banner() and exits
// 0 without any network access, so it is a safe way to capture the banner.
func runHelp(t *testing.T, extraEnv ...string) string {
	t.Helper()
	cmd := exec.Command("sh", installScript(t), "--help")
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh --help failed: %v\n%s", err, out)
	}
	return string(out)
}

// assertBareBanner checks that out carries no color codes and no leftover
// mascot dot-grid glyphs. The bare banner is now a short two-line header
// ("Handy Tools — one-line installer\n  <repo>") followed by the --help body.
func assertBareBanner(t *testing.T, label, out string) {
	t.Helper()
	if !strings.Contains(out, "Handy Tools") {
		t.Errorf("%s: banner missing the Handy Tools header:\n%q", label, head(out, 120))
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("%s: output contains an ANSI escape sequence:\n%q", label, head(out, 200))
	}
	if strings.Contains(out, "●") {
		t.Errorf("%s: output contains a mascot dot-grid glyph; the colored banner leaked", label)
	}
	// Sanity: prove --help actually ran past the banner, so a banner that
	// silently emitted nothing cannot pass this test.
	if !strings.Contains(out, "downloads the latest") {
		t.Errorf("%s: --help body missing; full output:\n%s", label, out)
	}
}

// TestBannerNoColorIsBare is the #33 check: with NO_COLOR=1 the installer
// banner collapses to the plain "Handy Tools installer" line — no ANSI codes,
// no mascot art.
func TestBannerNoColorIsBare(t *testing.T) {
	assertBareBanner(t, "NO_COLOR=1", runHelp(t, "NO_COLOR=1"))
}

// TestBannerNoColorFlagIsBare covers the equivalent --no-color flag path,
// which sets USE_COLOR=0 before usage() is reached.
func TestBannerNoColorFlagIsBare(t *testing.T) {
	cmd := exec.Command("sh", installScript(t), "--no-color", "--help")
	// Drop NO_COLOR/TERM from the env so only the flag is exercising the gate.
	cmd.Env = append(os.Environ(), "NO_COLOR=", "TERM=xterm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh --no-color --help failed: %v\n%s", err, out)
	}
	assertBareBanner(t, "--no-color", string(out))
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
