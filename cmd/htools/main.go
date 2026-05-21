// Command htools launches the Handy Tools Bubble Tea TUI.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/furkandedizkan/handy-tools/internal/buildinfo"
	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/ui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "doctor":
			runDoctor()
			return
		case "version", "--version", "-v":
			fmt.Println(buildinfo.String())
			return
		}
	}
	cfg, _, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config: ", err)
		os.Exit(1)
	}
	prog := tea.NewProgram(ui.New(cfg, queue.New()), tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
