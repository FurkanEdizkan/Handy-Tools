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

func toolPageFor(t *testing.T, id string) *toolPage {
	t.Helper()
	tl, ok := lookupTool(id)
	if !ok {
		t.Fatalf("tool %q not found in defaultTools", id)
	}
	return newToolPage(theme.Build(theme.Resolve("forge")), tl)
}

// TestToolPageExtractOptionsAreReal verifies the #163 change: the
// Archive-Extract option rows mutate real toolPage state, not literals.
func TestToolPageExtractOptionsAreReal(t *testing.T) {
	p := toolPageFor(t, "archive-extract")
	if got := p.optionCount(); got != 2 {
		t.Fatalf("extract optionCount = %d, want 2", got)
	}
	p.focusKind, p.focusIdx = focusOptions, 0
	ow := p.overwrite
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeySpace})
	if p.overwrite == ow {
		t.Fatal("space on extract row 0 should toggle overwrite")
	}
	p.focusIdx = 1
	amp := p.autoMultiPart
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeySpace})
	if p.autoMultiPart == amp {
		t.Fatal("space on extract row 1 should toggle autoMultiPart")
	}
}

// TestToolPagePDFOptionsAreReal verifies each PDF operation's option rows are
// bound to real state (#163): merge has none, render/split/text have working
// toggles and sliders.
func TestToolPagePDFOptionsAreReal(t *testing.T) {
	p := toolPageFor(t, "pdf")

	p.pdfop = pdfMerge
	if got := p.optionCount(); got != 0 {
		t.Fatalf("pdf merge optionCount = %d, want 0", got)
	}

	p.pdfop = pdfRender
	if got := p.optionCount(); got != 2 {
		t.Fatalf("pdf render optionCount = %d, want 2", got)
	}
	p.focusKind, p.focusIdx = focusOptions, 1
	jpeg := p.pdfJPEG
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeySpace})
	if p.pdfJPEG == jpeg {
		t.Fatal("space on render row 1 should toggle pdfJPEG")
	}

	p.pdfop = pdfSplit
	p.focusKind, p.focusIdx = focusOptions, 0
	n := p.pdfEveryN
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	if p.pdfEveryN <= n {
		t.Fatalf("'+' on split row 0 should raise pdfEveryN; %d → %d", n, p.pdfEveryN)
	}

	p.pdfop = pdfText
	p.focusKind, p.focusIdx = focusOptions, 0
	lay := p.pdfLayout
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeySpace})
	if p.pdfLayout == lay {
		t.Fatal("space on text row 0 should toggle pdfLayout")
	}
}

// TestRunCmdCarriesOptions confirms runCmd's RunJob carries the option payload
// the #153 runner needs.
func TestRunCmdCarriesOptions(t *testing.T) {
	p := imageToolPage(t)
	p.quality = 73
	p.overwrite = true

	msg := p.runCmd()()
	job, ok := msg.(RunJob)
	if !ok {
		t.Fatalf("runCmd message type = %T, want RunJob", msg)
	}
	if job.Quality != 73 {
		t.Fatalf("RunJob.Quality = %d, want 73", job.Quality)
	}
	if !job.Overwrite {
		t.Fatal("RunJob.Overwrite should be true")
	}
}
