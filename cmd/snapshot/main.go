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
	"github.com/furkandedizkan/handy-tools/internal/ui/mascot"
)

func main() {
	toStdout := flag.Bool("stdout", false, "print snapshots to stdout instead of writing files")
	width := flag.Int("width", 160, "terminal width to render at")
	height := flag.Int("height", 56, "terminal height to render at")
	outDir := flag.String("dir", "docs/screenshots", "output directory (relative to repo root)")
	brandDir := flag.String("brand-dir", "docs/brand", "output directory for plain-text brand art")
	flag.Parse()

	home := renderView(*width, *height, nil)
	tool := renderView(*width, *height, ui.OpenTool{ID: "convert-image"})
	wrenly := mascot.Sprite(mascot.CharacterWrenly)
	hopper := mascot.Sprite(mascot.CharacterHopper)

	if *toStdout {
		fmt.Println("== home ==")
		fmt.Println(home)
		fmt.Println()
		fmt.Println("== tool · convert images ==")
		fmt.Println(tool)
		fmt.Println()
		fmt.Println("== brand · wrenly ==")
		fmt.Println(wrenly)
		fmt.Println()
		fmt.Println("== brand · hopper ==")
		fmt.Println(hopper)
		return
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fail(err)
	}
	if err := os.MkdirAll(*brandDir, 0o755); err != nil {
		fail(err)
	}
	// 0o600 keeps gosec G306 quiet. The files are checked into git so the
	// effective on-disk mode after a fresh clone is set by the user's umask,
	// not by this WriteFile call.
	if err := os.WriteFile(filepath.Join(*outDir, "htools-home.txt"), []byte(home+"\n"), 0o600); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "htools-tool.txt"), []byte(tool+"\n"), 0o600); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(*brandDir, "wrenly.txt"), []byte(wrenly+"\n"), 0o600); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(*brandDir, "hopper.txt"), []byte(hopper+"\n"), 0o600); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s/htools-{home,tool}.txt and %s/{wrenly,hopper}.txt\n", *outDir, *brandDir)
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
