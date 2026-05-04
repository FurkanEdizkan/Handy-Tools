// Command handy launches the Bubble Tea TUI.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/furkandedizkan/handy/internal/config"
	"github.com/furkandedizkan/handy/internal/ui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		runDoctor()
		return
	}
	cfg, _, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config: ", err)
		os.Exit(1)
	}
	prog := tea.NewProgram(ui.New(cfg), tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
