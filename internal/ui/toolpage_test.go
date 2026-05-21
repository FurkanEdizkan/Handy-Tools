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

// imageToolPage builds a convert-image tool page for the file-input tests.
func imageToolPage(t *testing.T) *toolPage {
	t.Helper()
	tl, ok := lookupTool("convert-image")
	if !ok {
		t.Fatal("convert-image tool not found in defaultTools")
	}
	return newToolPage(theme.Build(theme.Resolve("forge")), tl)
}

// TestToolPageAddFileByPath drives the #160 path-entry flow: `b` opens the
// text field, typed runes accumulate, and Enter appends a real fileItem
// carrying an absolute Path.
func TestToolPageAddFileByPath(t *testing.T) {
	p := imageToolPage(t)
	before := len(p.files)

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if !p.capturingText() {
		t.Fatal("expected the path field to open after 'b'")
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/shot.png")})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if p.capturingText() {
		t.Fatal("capture should close after Enter")
	}
	if len(p.files) != before+1 {
		t.Fatalf("expected one new file, got %d → %d", before, len(p.files))
	}
	added := p.files[len(p.files)-1]
	if added.Path != "/tmp/shot.png" {
		t.Fatalf("Path = %q, want /tmp/shot.png", added.Path)
	}
	if added.Name != "shot.png" {
		t.Fatalf("Name = %q, want shot.png", added.Name)
	}
}

// TestToolPageCaptureCancel confirms Esc discards the buffer without adding a
// file.
func TestToolPageCaptureCancel(t *testing.T) {
	p := imageToolPage(t)
	before := len(p.files)

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("junk")})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if p.capturingText() {
		t.Fatal("Esc should close the capture")
	}
	if len(p.files) != before {
		t.Fatalf("Esc must not add a file; %d → %d", before, len(p.files))
	}
}

// TestToolPageEditCustomOutput confirms Enter on the Custom output row selects
// it and opens its path field, and a typed value replaces customPath.
func TestToolPageEditCustomOutput(t *testing.T) {
	p := imageToolPage(t)
	p.focusKind, p.focusIdx = focusOutDest, 2 // the Custom path row

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.capture != captureOutput {
		t.Fatalf("expected captureOutput after Enter on the Custom row, got %v", p.capture)
	}
	if p.out != outCustom {
		t.Fatal("Enter on the Custom row should select outCustom")
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlU}) // clear the prefilled buffer
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/out/dir")})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if p.customPath != "/out/dir" {
		t.Fatalf("customPath = %q, want /out/dir", p.customPath)
	}
}

// TestToolPageCaptureBackspace confirms Backspace trims the buffer.
func TestToolPageCaptureBackspace(t *testing.T) {
	p := imageToolPage(t)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.captureBuf != "ab" {
		t.Fatalf("captureBuf after Backspace = %q, want ab", p.captureBuf)
	}
}
