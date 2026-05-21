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
		"htools-gui is the Wails desktop build — compile it with: "+
			"go build -tags wails ./cmd/htools-gui  (needs CGO + libwebkit2gtk-4.0-dev + libgtk-3-dev)")
	os.Exit(1)
}
