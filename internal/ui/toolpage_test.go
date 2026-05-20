package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
)

// writeFakeExe drops an executable stub at path so exec.LookPath finds it.
func writeFakeExe(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write fake exe %s: %v", path, err)
	}
}

// TestDoctorPageReadsSysdep verifies the #84 change: the TUI Doctor page
// renders live sysdep.All() state rather than a hardcoded list, and a press
// of `r` re-probes PATH without restarting the TUI.
func TestDoctorPageReadsSysdep(t *testing.T) {
	styles := theme.Build(theme.Resolve("forge"))
	doc := tool{id: "doctor", glyph: "◊", label: "Doctor", mode: modeDoctor}

	// Reset the package cache when the test ends so a stale "not found"
	// entry from the empty-PATH phase can't leak into other tests.
	t.Cleanup(sysdep.Reset)

	// --- all missing: point PATH at an empty directory ---
	t.Setenv("PATH", t.TempDir())
	sysdep.Reset()

	p := newToolPage(styles, doc)
	out := p.View()

	mustContain(t, out, "0 / 5 found")
	for _, r := range sysdep.All() {
		mustContain(t, out, r.Tool.Name) // every Known tool has a row
	}
	mustContain(t, out, "MISSING")
	if strings.Contains(out, "FOUND") {
		t.Errorf("no tool should be FOUND with an empty PATH; view:\n%s", out)
	}
	// A missing tool shows its install hint for the running OS.
	if hint := sysdep.Known[0].InstallHint[runtime.GOOS]; hint != "" {
		mustContain(t, out, hint)
	}

	// --- one found: drop a fake `pdftotext` on PATH and press `r` ---
	binDir := t.TempDir()
	writeFakeExe(t, filepath.Join(binDir, "pdftotext"))
	t.Setenv("PATH", binDir)

	updated, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	p = updated
	out = p.View()

	mustContain(t, out, "1 / 5 found")
	mustContain(t, out, "FOUND")
}
