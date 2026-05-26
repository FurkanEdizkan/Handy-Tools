// Command handy is the user-facing front door for Handy Tools.
//
// It's a thin dispatcher: with no arguments it launches the desktop app
// (htools-gui); with a recognised subcommand it re-execs into one of the
// other four binaries (htools / htoolsd / htools-mcp) so flags and exit
// codes flow through unchanged. No tool logic lives here — adding a new
// CLI verb means editing cmd/htools, not this file.
//
// The four standalone binaries remain installed alongside handy and can
// still be invoked directly; handy is layered on top, not a replacement.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/furkandedizkan/handy-tools/internal/buildinfo"
)

// htoolsVerbs lists every first-positional argument that should be
// forwarded to the htools CLI (verb included). Keep this in sync with
// cmd/htools/main.go's dispatch() switch — if a verb shows up there but
// not here, `handy <verb>` will print "unknown subcommand".
var htoolsVerbs = map[string]bool{
	"convert":    true,
	"pack":       true,
	"extract":    true,
	"pdf":        true,
	"hash":       true,
	"rename":     true,
	"diff-tree":  true,
	"strip-meta": true,
	"doctor":     true,
}

// decision enumerates what main() should do with the parsed args. The
// route() function returns this so its logic is pure (no I/O), which
// makes table-driven unit tests trivial.
type decision int

const (
	decExecBackend decision = iota
	decPrintHelp
	decPrintVersion
	decUnknown
)

type routeResult struct {
	Decision decision
	Backend  string   // basename of the backend to exec, when Decision == decExecBackend
	Args     []string // args to pass to the backend
	BadVerb  string   // the unknown verb, when Decision == decUnknown
}

// route is a pure function so it can be unit-tested without touching the
// filesystem or process state. main() turns the routeResult into actual
// I/O and a syscall.Exec.
func route(args []string) routeResult {
	if len(args) == 0 {
		return routeResult{Decision: decExecBackend, Backend: "htools-gui"}
	}
	head := args[0]
	rest := args[1:]
	switch head {
	case "--version", "-v", "version":
		return routeResult{Decision: decPrintVersion}
	case "--help", "-h", "help":
		return routeResult{Decision: decPrintHelp}
	case "gui":
		return routeResult{Decision: decExecBackend, Backend: "htools-gui", Args: rest}
	case "serve", "daemon":
		return routeResult{Decision: decExecBackend, Backend: "htoolsd", Args: rest}
	case "mcp":
		return routeResult{Decision: decExecBackend, Backend: "htools-mcp", Args: rest}
	}
	if htoolsVerbs[head] {
		// Forward the verb itself so htools sees the same argv it would
		// have seen as a direct invocation.
		return routeResult{Decision: decExecBackend, Backend: "htools", Args: args}
	}
	return routeResult{Decision: decUnknown, BadVerb: head}
}

func helpText() string {
	return `handy — the front door for Handy Tools

Usage:
  handy                    Launch the desktop app (same as: handy gui)
  handy <verb> [args...]   Run a CLI tool, daemon, or MCP server

Terminal verbs (re-execs htools):
  convert      Convert images between formats
  pack         Create archives
  extract      Extract archives
  pdf          PDF merge / split / render / text
  hash         File hashes (md5 / sha256 / blake3)
  rename       Batch-rename files with a regex
  diff-tree    Compare two directory trees
  strip-meta   Remove EXIF/IPTC/XMP from images
  doctor       Report which optional system tools are available

Server verbs:
  serve        Run the gRPC + HTTP/SSE server (re-execs htoolsd)
  mcp          Run the Model Context Protocol server over stdio (re-execs htools-mcp)
  gui          Open the desktop app explicitly (re-execs htools-gui)

Other:
  --version    Print version and exit
  --help       Show this help

Each verb's flags live on the underlying binary; run it directly to see them:
  htools --help     htoolsd --help     htools-mcp --help     htools-gui --help
`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main() factored for testability: it returns the exit code rather
// than calling os.Exit so a test harness can drive it without subprocess
// machinery. The actual binary entry point just wraps it with os.Exit.
func run(args []string, stdout, stderr io.Writer) int {
	r := route(args)
	switch r.Decision {
	case decPrintVersion:
		fmt.Fprintln(stdout, buildinfo.String())
		return 0
	case decPrintHelp:
		fmt.Fprint(stdout, helpText())
		return 0
	case decUnknown:
		fmt.Fprintf(stderr, "handy: unknown subcommand %q\n\n", r.BadVerb)
		fmt.Fprint(stderr, helpText())
		return 2
	case decExecBackend:
		return execBackend(r.Backend, r.Args, stderr)
	}
	// Unreachable; the compiler can't see that decision is exhaustive.
	return 1
}

func execBackend(backend string, args []string, stderr io.Writer) int {
	// Headless guard: if we're about to launch the GUI on a Linux box
	// with no display, bail out with help instead of crashing inside
	// WebKit. macOS always has a display so the guard is Linux-only.
	if backend == "htools-gui" && isHeadless() {
		fmt.Fprintln(stderr, "handy: no display detected ($DISPLAY and $WAYLAND_DISPLAY both empty)")
		fmt.Fprintln(stderr, "       skipping GUI launch; run `handy --help` for terminal verbs.")
		return 1
	}

	path, err := locateBackend(backend)
	if err != nil {
		fmt.Fprint(stderr, backendNotFoundHint(backend, err))
		return 127
	}

	// argv[0] should be the backend's basename, not the absolute path —
	// that way tools see themselves under the expected name in os.Args[0]
	// and any os.Executable-based path lookups they do still work via the
	// kernel's record of the real path.
	argv := append([]string{backend}, args...)
	// syscall.Exec replaces the current process; everything after it only
	// runs on failure. Stdin/stdout/stderr/signals/exit code all flow
	// through the kernel directly.
	if err := syscall.Exec(path, argv, os.Environ()); err != nil {
		fmt.Fprintln(stderr, "handy: exec:", err)
		return 126
	}
	return 0 // unreachable
}

// isHeadless reports whether we likely have no graphical session. Used to
// short-circuit the bare-`handy` GUI launch in headless environments
// (ssh, CI, containers) so the user gets help instead of an opaque
// WebKit failure.
func isHeadless() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	return os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

// backendNotFoundHint turns a backend-lookup failure into a user-facing
// message. The generic message is fine for the four CGO-free backends —
// they're all shipped together in the default tarball, so "not present"
// almost always means the install was tampered with. htools-gui is
// different: it ships in its own platform-gated tarball and isn't
// produced by a default `make build` (Wails needs CGO + libwebkit2gtk),
// so a tailored hint saves users a trip to the docs.
func backendNotFoundHint(backend string, err error) string {
	if backend != "htools-gui" {
		return fmt.Sprintf("handy: %s\n", err)
	}
	return `handy: couldn't find "htools-gui" — the desktop app isn't installed in this layout.

If you installed via install.sh on linux/amd64, the GUI download was skipped
or failed. Re-run install.sh, or grab the GUI tarball from
https://github.com/FurkanEdizkan/Handy-Tools/releases.

If you're running from source, build the GUI separately:
  sudo apt install libwebkit2gtk-4.1-dev   # or 4.0-dev on Ubuntu 22.04
  make gui-build                            # produces bin/htools-gui
`
}

// locateBackend finds the named backend binary. Resolution order:
//  1. Same directory as this handy executable (covers the standard
//     install where all five binaries land in $HOME/.local/bin together).
//  2. exec.LookPath (handles scattered installs / dev environments).
func locateBackend(name string) (string, error) {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), name)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf(
		"couldn't find %q alongside handy or on $PATH — reinstall, or invoke %q directly",
		name, name,
	)
}
