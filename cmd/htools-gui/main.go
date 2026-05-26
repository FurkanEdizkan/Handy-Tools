//go:build wails

// Command htools-gui wraps the Handy Tools Svelte frontend and Go core in a
// native desktop window via Wails. It serves the same embedded SPA and /v1/*
// HTTP API as htoolsd, but through an in-process Wails AssetServer — there is
// no network listener.
package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"regexp"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	httpapi "github.com/furkandedizkan/handy-tools/internal/api/http"
	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/server"
)

// webkitNoise matches the two stderr lines WebKit's WTF prints at GC
// signal-handler install time:
//
//	Overriding existing handler for signal 10. Set JSC_SIGNAL_FOR_GC
//	if you want WebKit to use a different signal
//	ERROR: invalid option: JSC_SIGNAL_FOR_GC=<n>
//
// The override is harmless (JSC's handler wins and the Go runtime
// doesn't depend on SIGUSR1), and the "invalid option" line is JSC's
// own option parser complaining about an env-var name it doesn't
// recognise (despite the message above suggesting it). Both are
// noise to the end user. Setting JSC_SIGNAL_FOR_GC to a different
// signal just shifts the warning to a different number — every
// signal slot Go and glib touch triggers it — and adds the
// "invalid option" line on top, so env-var workarounds make it worse.
// Filtering at the fd-2 level catches both regardless of where they
// come from.
var webkitNoise = regexp.MustCompile(
	`^(Overriding existing handler for signal \d+|ERROR: invalid option: JSC_)`)

// init replaces fd 2 with the write end of a pipe before main() runs.
// A goroutine drains the pipe, drops every line matching webkitNoise,
// and writes the rest back to the original stderr fd. Has to happen
// before wails.Run loads WebKit (the libraries print to fd 2 directly
// via fprintf, so Go's os.Stderr wrapper can't intercept them).
//
// init() is the right hook: it runs after CGO load for this package
// but before main() — and WebKit's signal-handler install fires inside
// wails.Run, which is called from main(). The pipe is set up exactly
// once; Go's runtime doesn't unwind init goroutines on exit, but the
// kernel cleans up the fds when the process dies.
func init() {
	originalFd, err := syscall.Dup(2)
	if err != nil {
		return // can't filter; let the noise through, don't crash startup
	}
	original := os.NewFile(uintptr(originalFd), "stderr")

	r, w, err := os.Pipe()
	if err != nil {
		original.Close()
		return
	}
	if err := syscall.Dup2(int(w.Fd()), 2); err != nil {
		original.Close()
		r.Close()
		w.Close()
		return
	}
	// Keep os.Stderr pointing at fd 2 so the rest of the Go program
	// writes through the same filter. log.Default already targets fd 2.
	os.Stderr = os.NewFile(2, "stderr")

	go filterStderrNoise(r, original)
}

// filterStderrNoise scans r line-by-line, drops anything matching
// webkitNoise, and forwards the rest to w (the real stderr). Runs for
// the lifetime of the process.
func filterStderrNoise(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1<<16)
	for scanner.Scan() {
		line := scanner.Bytes()
		if webkitNoise.Match(line) {
			continue
		}
		_, _ = w.Write(append(line, '\n'))
	}
}

func main() {
	// A desktop app runs on the user's own machine, so there is no path
	// sandbox: AllowRoots is "/" and the native file picker (#80) hands the
	// tools real absolute paths the user already has access to.
	api := httpapi.New(server.Options{AllowRoots: []string{"/"}}, queue.New())

	app := newApp()
	if err := wails.Run(&options.App{
		Title:  "Handy Tools",
		Width:  1100,
		Height: 720,
		AssetServer: &assetserver.Options{
			// internal/api/http already serves the embedded Svelte SPA and the
			// /v1/* API from one mux; the webview talks straight to it.
			Handler: api.Handler(),
		},
		Menu:      appMenu(app),
		OnStartup: app.startup,
		Bind:      []any{app},
	}); err != nil {
		log.Fatalf("htools-gui: %v", err)
	}
}
