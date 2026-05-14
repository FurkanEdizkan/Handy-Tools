// Command snapshot renders the htools TUI views into plain-text files under
// docs/screenshots/. It exists so contributors can regenerate the README
// preview blocks deterministically after touching the UI; it is not part of
// the user-facing release (excluded from .goreleaser.yaml's builds list).
//
// Usage:
//
//	go run ./cmd/snapshot          # writes docs/screenshots/*.txt
//	go run ./cmd/snapshot -stdout  # prints to stdout instead
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/ui"
)

func main() {
	toStdout := flag.Bool("stdout", false, "print snapshots to stdout instead of writing files")
	width := flag.Int("width", 160, "terminal width to render at")
	height := flag.Int("height", 56, "terminal height to render at")
	outDir := flag.String("dir", "docs/screenshots", "output directory (relative to repo root)")
	flag.Parse()

	home := renderView(*width, *height, nil)
	tool := renderView(*width, *height, ui.OpenTool{ID: "convert-image"})

	if *toStdout {
		fmt.Println("== home ==")
		fmt.Println(home)
		fmt.Println()
		fmt.Println("== tool · convert images ==")
		fmt.Println(tool)
		return
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "htools-home.txt"), []byte(home+"\n"), 0o644); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "htools-tool.txt"), []byte(tool+"\n"), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s/htools-home.txt and %s/htools-tool.txt\n", *outDir, *outDir)
}

// renderView builds the model, dispatches an optional initial message, and
// returns the rendered View() string.
func renderView(width, height int, msg tea.Msg) string {
	m := ui.New(config.Defaults())
	u, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	mm := u.(ui.Model)
	if msg != nil {
		u, _ = mm.Update(msg)
		mm = u.(ui.Model)
		// re-deliver the window size so the freshly-instantiated tool page
		// picks up its right-column width
		u, _ = mm.Update(tea.WindowSizeMsg{Width: width, Height: height})
		mm = u.(ui.Model)
	}
	return mm.View()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "snapshot: ", err)
	os.Exit(1)
}
