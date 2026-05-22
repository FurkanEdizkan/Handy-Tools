//go:build !wails

// Command htools-gui is the Handy Tools desktop application (Wails). The real
// implementation is gated behind the `wails` build tag because it needs CGO
// and the webkit2gtk toolchain; this stub stands in for the default toolchain
// so `go build ./...`, `go test ./...` and the standard CI jobs stay
// CGO-free. CI's dedicated `gui-build` job compiles the real binary with
// `-tags wails`. See wails.json and main.go.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr,
		"htools-gui is the Wails desktop build — build and run it with:  make gui\n"+
			"It needs CGO plus the GTK/webkit dev headers:\n"+
			"  Ubuntu 24.04+:  sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev\n"+
			"  Ubuntu 22.04:   sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev\n"+
			"make gui picks the matching webkit build tag automatically.")
	os.Exit(1)
}
