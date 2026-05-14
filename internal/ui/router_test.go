package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/ui/mascot"
)

// TestViewRendersHomeAndToolPages exercises the design's home → tool → home
// flow without a real terminal. It catches layout regressions (missing
// brand text, broken queue header) and asserts that OpenTool/GoHome wires
// switch the right-column page as expected.
func TestViewRendersHomeAndToolPages(t *testing.T) {
	m := New(config.Defaults())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	m = updated.(Model)

	out := m.View()
	mustContain(t, out, "HANDY TOOLS")
	mustContain(t, out, "AVAILABLE TOOLS")
	mustContain(t, out, "Convert images")
	mustContain(t, out, "QUEUE")
	mustContain(t, out, "wrenly")

	// Open the image tool. The right column should swap to the tool page,
	// the breadcrumb should follow, and the run row + summary should render.
	updated, _ = m.Update(OpenTool{ID: "convert-image"})
	m = updated.(Model)
	if m.current != PageTool {
		t.Fatalf("expected PageTool after OpenTool, got %v", m.current)
	}
	out = m.View()
	mustContain(t, out, "Convert images")
	mustContain(t, out, "Drop")
	mustContain(t, out, "OUTPUT DESTINATION")
	mustContain(t, out, "RUN")

	// Back to home.
	updated, _ = m.Update(GoHome{})
	m = updated.(Model)
	if m.current != PageHome {
		t.Fatalf("expected PageHome after GoHome, got %v", m.current)
	}
}

// TestQueueRendersSeededJobs makes sure the design's pre-populated queue
// (done + failed + queued) shows up in the left column. We check for tokens
// that survive lipgloss soft-wrapping in the narrow queue column — labels
// like "invoice-2026-04.png" get broken across lines so we look at the
// expanded log content and status pills instead.
func TestQueueRendersSeededJobs(t *testing.T) {
	m := New(config.Defaults())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	m = updated.(Model)
	out := m.View()
	mustContain(t, out, "QUEUE")
	mustContain(t, out, "DONE")
	mustContain(t, out, "FAIL")
	mustContain(t, out, "WAIT")
	mustContain(t, out, "pdftoppm") // proves the failed-job log expansion rendered
	mustContain(t, out, "MISSING_BINARY")
}

// TestRunJobUpdatesState confirms that dispatching a RunJob enqueues a
// running job and surfaces it in the state block + queue.
func TestRunJobUpdatesState(t *testing.T) {
	m := New(config.Defaults())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	m = updated.(Model)
	updated, _ = m.Update(OpenTool{ID: "convert-image"})
	m = updated.(Model)

	if m.tool == nil {
		t.Fatal("expected toolPage to be active")
	}
	updated, _ = m.Update(RunJob{
		Tool:    m.tool.tool,
		Files:   m.tool.files,
		Summary: m.tool.summary(),
	})
	m = updated.(Model)
	if !m.running {
		t.Fatal("expected model.running=true after RunJob")
	}
	if m.mascot.State() != mascot.StateThinking {
		t.Fatalf("expected mascot.Thinking after RunJob, got %v", m.mascot.State())
	}
	if len(m.queue) == 0 || m.queue[0].Status != JobRunning {
		t.Fatalf("expected first queue entry to be running, got %+v", m.queue)
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("rendered view missing %q\n---\n%s\n---", needle, haystack)
	}
}
