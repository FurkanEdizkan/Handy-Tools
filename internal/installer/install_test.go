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

// TestUninstallRemovesBinariesAndConfig drops fake htools / htoolsd /
// htools-gui binaries into a temp install dir plus a default-location config
// dir ($XDG_CONFIG_HOME/handy-tools), runs install.sh --uninstall --yes, and
// asserts everything is gone.
func TestUninstallRemovesBinariesAndConfig(t *testing.T) {
	tmpBin := t.TempDir()
	xdg := t.TempDir()
	cfgDir := filepath.Join(xdg, "handy-tools")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	for _, b := range []string{"htools", "htoolsd", "htools-gui"} {
		p := filepath.Join(tmpBin, b)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("seed %s: %v", b, err)
		}
	}

	out := runUninstall(t, tmpBin, "XDG_CONFIG_HOME="+xdg, "HANDY_TOOLS_CONFIG=")

	for _, b := range []string{"htools", "htoolsd", "htools-gui"} {
		p := filepath.Join(tmpBin, b)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still present after uninstall: err=%v", p, err)
		}
	}
	if _, err := os.Stat(cfgDir); !os.IsNotExist(err) {
		t.Errorf("config dir %s still present: err=%v\n%s", cfgDir, err, out)
	}
}

// TestUninstallRemovesOnlyTheCustomConfigFile is a regression for uninstall
// running `rm -rf "$(dirname "$HANDY_TOOLS_CONFIG")"`: a config kept at
// ~/dotfiles/handy.yaml took the whole ~/dotfiles with it (and
// $HOME/handy.yaml meant `rm -rf $HOME`). Only the file itself may go.
func TestUninstallRemovesOnlyTheCustomConfigFile(t *testing.T) {
	tmpBin := t.TempDir()
	dotfiles := t.TempDir()
	cfg := filepath.Join(dotfiles, "handy.yaml")
	sibling := filepath.Join(dotfiles, "zshrc")
	for _, p := range []string{cfg, sibling} {
		if err := os.WriteFile(p, []byte("# keep?\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := runUninstall(t, tmpBin, "HANDY_TOOLS_CONFIG="+cfg)

	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Errorf("custom config %s still present: err=%v\n%s", cfg, err, out)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("uninstall deleted an unrelated file next to the config: %v\n%s", err, out)
	}
}

// runUninstall runs `install.sh --uninstall --yes --dir bin` with extra env and
// returns its output, failing the test on a non-zero exit.
func runUninstall(t *testing.T, bin string, extraEnv ...string) string {
	t.Helper()
	cmd := exec.Command("sh", installScript(t), "--uninstall", "--yes", "--dir", bin)
	cmd.Env = append(os.Environ(), "NO_COLOR=1") // no ANSI noise in test output
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh --uninstall --yes failed: %v\n%s", err, out)
	}
	return string(out)
}

// TestUninstallAbortsOnDeclinedPrompt verifies that without --yes the
// uninstaller never takes a yes it didn't get: here stdin is a pipe (as under
// `curl … | sh -s -- --uninstall`, where stdin is the script itself) and there
// is no terminal to ask on, so it must exit non-zero and leave files untouched.
func TestUninstallAbortsOnDeclinedPrompt(t *testing.T) {
	tmpBin := t.TempDir()
	tmpCfgDir := t.TempDir()
	tmpCfg := filepath.Join(tmpCfgDir, "config.yaml")
	binPath := filepath.Join(tmpBin, "htools")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.WriteFile(tmpCfg, []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cmd := exec.Command("sh", installScript(t), "--uninstall", "--dir", tmpBin)
	cmd.Env = append(os.Environ(),
		"HANDY_TOOLS_CONFIG="+tmpCfg,
		"NO_COLOR=1",
	)
	cmd.Stdin = strings.NewReader("y\n") // a piped "y" is not a terminal answer
	detachFromTTY(cmd)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("declined uninstall should exit non-zero; output:\n%s", out)
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Errorf("declined uninstall removed binary: %v", err)
	}
	if _, err := os.Stat(tmpCfg); err != nil {
		t.Errorf("declined uninstall removed config: %v", err)
	}
}
