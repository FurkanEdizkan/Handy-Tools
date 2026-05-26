package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// handyBinaryPath is populated by TestMain — every subtest copies it next
// to a stub backend so the same-dir backend lookup works the same way as
// a real install.
var handyBinaryPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "handy-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mktemp:", err)
		os.Exit(2)
	}
	defer os.RemoveAll(dir)
	handyBinaryPath = filepath.Join(dir, "handy")
	build := exec.Command("go", "build", "-o", handyBinaryPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build handy: %v\n%s\n", err, out)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

// writeStub creates a POSIX-sh stub at dir/name that prints its argv to
// stderr in a recognisable format and exits 0. The project's only supported
// targets are linux and darwin, so a shell script is portable enough.
func writeStub(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	body := "#!/bin/sh\nprintf 'STUB:%s' \"$0\"\nfor a in \"$@\"; do printf ' %s' \"$a\"; done\nprintf '\\n'\nexit 0\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil { //nolint:gosec // test-only stub
		t.Fatalf("write stub %s: %v", name, err)
	}
	return p
}

// installLayout copies the built handy into a fresh temp dir, drops a
// stub binary for each requested backend alongside it, and returns the
// path of the handy copy. Mirrors the "all five binaries in one dir"
// install layout that locateBackend's same-dir lookup is designed for.
func installLayout(t *testing.T, stubs ...string) string {
	t.Helper()
	dir := t.TempDir()
	handyCopy := filepath.Join(dir, "handy")
	src, err := os.ReadFile(handyBinaryPath) //nolint:gosec // path is built by TestMain
	if err != nil {
		t.Fatalf("read built handy: %v", err)
	}
	if err := os.WriteFile(handyCopy, src, 0o755); err != nil { //nolint:gosec // test-only copy
		t.Fatalf("write handy copy: %v", err)
	}
	for _, s := range stubs {
		writeStub(t, dir, s)
	}
	return handyCopy
}

// runHandy invokes the test-layout handy with the given args and an empty
// PATH (so any backend lookup must hit the same-dir branch — not whatever
// htools happens to be on the developer's $PATH). DISPLAY and WAYLAND_DISPLAY
// are cleared so the headless guard fires on bare-handy. Stdin is closed.
func runHandy(t *testing.T, handy string, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	return runHandyWithEnv(t, handy, map[string]string{
		"PATH":            "",
		"DISPLAY":         "",
		"WAYLAND_DISPLAY": "",
	}, args...)
}

// runHandyWithEnv is the lower-level form. The caller decides what to do
// with PATH / DISPLAY / WAYLAND_DISPLAY by passing them in the overrides
// map (a "" value clears the variable). Used by the GUI-not-found subtest
// which needs DISPLAY set so the headless guard doesn't short-circuit.
func runHandyWithEnv(t *testing.T, handy string, overrides map[string]string, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(handy, args...) //nolint:gosec // handy path is built by TestMain into a temp dir
	env := []string{}
	for _, kv := range os.Environ() {
		key := kv
		if i := strings.IndexByte(kv, '='); i > 0 {
			key = kv[:i]
		}
		if _, override := overrides[key]; override {
			continue
		}
		env = append(env, kv)
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return out.String(), errb.String(), ee.ExitCode()
		}
		t.Fatalf("run handy: %v\nstdout: %s\nstderr: %s", err, out.String(), errb.String())
	}
	return out.String(), errb.String(), 0
}

func TestE2E_Version(t *testing.T) {
	handy := installLayout(t)
	stdout, stderr, exit := runHandy(t, handy, "--version")
	if exit != 0 {
		t.Fatalf("exit: got %d, want 0 (stderr: %s)", exit, stderr)
	}
	if stdout == "" {
		t.Fatal("expected version on stdout")
	}
	// buildinfo.String renders like "0.0.0-dev linux/amd64".
	if !strings.Contains(stdout, "/") {
		t.Fatalf("version output doesn't look like 'VERSION OS/ARCH': %q", stdout)
	}
}

func TestE2E_Help(t *testing.T) {
	handy := installLayout(t)
	stdout, _, exit := runHandy(t, handy, "--help")
	if exit != 0 {
		t.Fatalf("exit: got %d, want 0", exit)
	}
	for _, want := range []string{"convert", "serve", "mcp", "gui", "doctor"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing %q; got:\n%s", want, stdout)
		}
	}
}

func TestE2E_UnknownVerb(t *testing.T) {
	handy := installLayout(t)
	_, stderr, exit := runHandy(t, handy, "frobnicate")
	if exit != 2 {
		t.Fatalf("exit: got %d, want 2", exit)
	}
	if !strings.Contains(stderr, "unknown subcommand") {
		t.Errorf("stderr missing 'unknown subcommand'; got:\n%s", stderr)
	}
}

func TestE2E_HeadlessGuardBlocksGUI(t *testing.T) {
	// installLayout puts a stub htools-gui alongside, but the headless
	// guard runs *before* the backend lookup, so the stub never gets
	// invoked. We still write it to prove that's what's blocking the call
	// (without the guard, the stub would run and exit 0).
	handy := installLayout(t, "htools-gui")
	_, stderr, exit := runHandy(t, handy /* no args */)
	if exit != 1 {
		t.Fatalf("exit: got %d, want 1 (stderr: %s)", exit, stderr)
	}
	if !strings.Contains(stderr, "no display detected") {
		t.Errorf("stderr missing 'no display detected'; got:\n%s", stderr)
	}
}

func TestE2E_DispatchToHtools(t *testing.T) {
	handy := installLayout(t, "htools")
	// `handy doctor` re-execs `htools doctor`. The stub prints its own
	// path + every arg, so we can see both the backend the dispatcher
	// chose and that the verb was forwarded.
	stdout, stderr, exit := runHandy(t, handy, "doctor")
	if exit != 0 {
		t.Fatalf("exit: got %d, want 0 (stderr: %s)", exit, stderr)
	}
	// stub prints to stdout
	if !strings.Contains(stdout, "STUB:") || !strings.HasSuffix(strings.TrimSpace(stdout), " doctor") {
		t.Fatalf("expected stub htools to be invoked with `doctor`; got:\n%s", stdout)
	}
}

func TestE2E_DispatchToHtoolsd(t *testing.T) {
	handy := installLayout(t, "htoolsd")
	stdout, stderr, exit := runHandy(t, handy, "serve", "--listen", ":7777")
	if exit != 0 {
		t.Fatalf("exit: got %d, want 0 (stderr: %s)", exit, stderr)
	}
	if !strings.Contains(stdout, "--listen :7777") {
		t.Fatalf("expected stub htoolsd to see forwarded flags; got:\n%s", stdout)
	}
}

func TestE2E_BackendNotFound(t *testing.T) {
	// Install layout has no htools stub; routing to it should fail with
	// a structured error and exit 127.
	handy := installLayout(t /* no stubs */)
	_, stderr, exit := runHandy(t, handy, "doctor")
	if exit != 127 {
		t.Fatalf("exit: got %d, want 127", exit)
	}
	if !strings.Contains(stderr, "couldn't find") {
		t.Errorf("stderr missing 'couldn't find'; got:\n%s", stderr)
	}
	// Generic backends must NOT leak the GUI-specific hint text.
	if strings.Contains(stderr, "make gui-build") {
		t.Errorf("htools not-found message leaked GUI hint; got:\n%s", stderr)
	}
}

// TestE2E_GUIBackendNotFound runs `handy gui` with no htools-gui stub in
// the install dir, with DISPLAY set so the headless guard doesn't fire
// first. We're testing the specialised hint that points fresh-install
// users at install.sh and source-build users at `make gui-build`.
func TestE2E_GUIBackendNotFound(t *testing.T) {
	handy := installLayout(t /* no stubs at all */)
	_, stderr, exit := runHandyWithEnv(t, handy, map[string]string{
		"PATH":            "",
		"DISPLAY":         ":99", // any non-empty value bypasses the guard
		"WAYLAND_DISPLAY": "",
	}, "gui")
	if exit != 127 {
		t.Fatalf("exit: got %d, want 127 (stderr: %s)", exit, stderr)
	}
	for _, want := range []string{"htools-gui", "install.sh", "make gui-build", "libwebkit2gtk"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("GUI not-found hint missing %q; got:\n%s", want, stderr)
		}
	}
}
