package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/ui/mascot"
)

// drainQueue folds queue events into m (via the same queueEventMsg the live
// SubscribeAll bridge produces) until the most-recent job reaches a terminal
// state. Same-package access to the unexported events channel lets a test
// drive the async pipeline deterministically without a Bubble Tea runtime.
func drainQueue(t *testing.T, m Model) Model {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-m.events:
			if !ok {
				return m
			}
			u, _ := m.Update(queueEventMsg{job: ev})
			m = u.(Model)
			if len(m.queue) > 0 {
				switch m.queue[0].Status {
				case JobDone, JobFailed:
					return m
				}
			}
		case <-deadline:
			t.Fatal("timed out draining queue events")
			return m
		}
	}
}

// TestTabCyclePersistsTheme verifies the #90 fix: cycling the theme with Tab
// on the home page writes the choice to disk so it survives the next launch.
func TestTabCyclePersistsTheme(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))

	m := New(config.Defaults(), nil) // theme defaults to "forge"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	if m.cfg.Theme.Name != "snow" {
		t.Fatalf("in-memory theme after Tab = %q, want snow", m.cfg.Theme.Name)
	}
	reloaded, _, err := config.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Theme.Name != "snow" {
		t.Fatalf("persisted theme = %q, want snow (Tab choice did not survive a reload)", reloaded.Theme.Name)
	}
}

// TestViewRendersHomeAndToolPages exercises the design's home → tool → home
// flow without a real terminal. It catches layout regressions (missing
// brand text, broken queue header) and asserts that OpenTool/GoHome wires
// switch the right-column page as expected.
func TestViewRendersHomeAndToolPages(t *testing.T) {
	m := New(config.Defaults(), nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	m = updated.(Model)

	out := m.View()
	mustContain(t, out, "HANDY TOOLS")
	mustContain(t, out, "AVAILABLE TOOLS")
	mustContain(t, out, "Convert images")
	mustContain(t, out, "QUEUE")
	mustContain(t, out, "STATE")  // state block label
	mustContain(t, out, "wrenly") // mascot character chip

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

// TestQueueRendersJobFromQueue runs a job through the real shared queue and
// asserts it surfaces in the left-column queue panel — exercising the whole
// Enqueue → SubscribeAll → queueEventMsg → render pipeline.
func TestQueueRendersJobFromQueue(t *testing.T) {
	m := New(config.Defaults(), queue.New())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	m = updated.(Model)

	// A fresh TUI has no seeded jobs — the queue panel is empty but present.
	mustContain(t, m.View(), "QUEUE")

	updated, _ = m.Update(OpenTool{ID: "convert-image"})
	m = updated.(Model)
	updated, _ = m.Update(RunJob{
		Tool:    m.tool.tool,
		Files:   m.tool.files,
		Summary: m.tool.summary(),
	})
	m = updated.(Model)

	m = drainQueue(t, m)
	if m.queue[0].Status != JobDone {
		t.Fatalf("expected job to reach done, got %+v", m.queue[0])
	}
	out := m.View()
	mustContain(t, out, "QUEUE")
	mustContain(t, out, "DONE") // the done status pill rendered
}

// TestRunJobUpdatesState confirms dispatching a RunJob optimistically marks
// the model running and, once queue events drain, flips the job to done.
func TestRunJobUpdatesState(t *testing.T) {
	m := New(config.Defaults(), queue.New())
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
	if len(m.queue) == 0 || m.queue[0].ID == "" {
		t.Fatalf("expected an optimistic queue row after RunJob, got %+v", m.queue)
	}

	m = drainQueue(t, m)
	if m.queue[0].Status != JobDone {
		t.Fatalf("expected first queue entry done after drain, got %+v", m.queue[0])
	}
	if m.running {
		t.Fatal("expected model.running=false after the job completed")
	}
}

// TestSettingsPopoverTogglesAndCycles confirms the design's top-left
// settings popover opens on ',', cycles the theme via enter, and closes on
// esc — replacing the old PageSettings navigation flow.
func TestSettingsPopoverTogglesAndCycles(t *testing.T) {
	m := New(config.Defaults(), nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	m = updated.(Model)

	// Closed by default — popover title shouldn't appear in the render.
	if strings.Contains(m.View(), "SETTINGS") {
		t.Fatal("popover should not render before ',' is pressed")
	}

	// Press ',' to open. The popover title and its rows should now be visible.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}})
	m = updated.(Model)
	if !m.pop.IsOpen() {
		t.Fatal("expected popover open after ','")
	}
	out := m.View()
	mustContain(t, out, "SETTINGS")
	mustContain(t, out, "Theme")
	mustContain(t, out, "Scanlines")
	mustContain(t, out, "Mascot")
	mustContain(t, out, "htoolsd")

	// Cursor is on the Theme row. Cycling forge → snow updates cfg + styles.
	if got := m.cfg.Theme.Name; got != "forge" {
		t.Fatalf("expected initial theme forge, got %q", got)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if got := m.cfg.Theme.Name; got != "snow" {
		t.Fatalf("expected theme snow after enter, got %q", got)
	}

	// Move down to Mascot row and cycle — the character chip should flip.
	for i := 0; i < 2; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if got := m.cfg.Mascot.Style; got != "hopper" {
		t.Fatalf("expected mascot hopper after cycle, got %q", got)
	}
	if got := m.mascot.Character(); got != "hopper" {
		t.Fatalf("mascot did not sync to hopper, got %q", got)
	}

	// Esc closes.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.pop.IsOpen() {
		t.Fatal("expected popover closed after esc")
	}
}

// longLogJob is a synthetic failed job whose expanded stderr panel carries ten
// log lines — the fixture TestViewFitsSmallTerminal uses to exercise the #53
// stderr-cap behaviour now that the queue is no longer pre-seeded.
func longLogJob() Job {
	logs := make([]LogLine, 0, 10)
	for i := 1; i <= 10; i++ {
		logs = append(logs, LogLine{T: "14:08:22.0" + string(rune('0'+i%10)), Lvl: "INFO",
			Msg: "render step detail line number " + string(rune('0'+i%10))})
	}
	return Job{
		ID: "long", Label: "manual.pdf → 32 pages", Kind: "pdf",
		Status: JobFailed, Time: "14:08", Err: "pdftoppm not found",
		Expanded: true, Logs: logs,
	}
}

// TestViewFitsSmallTerminal regression-tests #53: a failed job's expanded
// stderr panel of ten unwrapped log lines previously soft-wrapped into 30+
// visual rows in the narrow queue column, pushing the header and home menu
// off the top of a 30-row terminal. We assert that on a small terminal the
// expanded panel is capped (carries the "earlier" elision marker) and on a
// large terminal the cap does not engage.
func TestViewFitsSmallTerminal(t *testing.T) {
	m := New(config.Defaults(), nil)
	m.queue = []Job{longLogJob()}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 124, Height: 30})
	m = updated.(Model)
	smallOut := m.View()

	mustContain(t, smallOut, "HANDY TOOLS")     // header still emitted
	mustContain(t, smallOut, "AVAILABLE TOOLS") // home menu still emitted
	mustContain(t, smallOut, "earlier")         // log expansion was capped

	smallLines := strings.Count(smallOut, "\n")
	if smallLines > 32 {
		t.Fatalf("View at 124×30 produced %d lines; expected ≤ 32", smallLines)
	}

	// At a tall terminal the cap should not engage — all 10 log lines fit,
	// so the "earlier" marker shouldn't appear.
	big := New(config.Defaults(), nil)
	big.queue = []Job{longLogJob()}
	updated, _ = big.Update(tea.WindowSizeMsg{Width: 160, Height: 60})
	big = updated.(Model)
	bigOut := big.View()
	if strings.Contains(bigOut, "earlier") {
		t.Fatal("View at 160×60 should fit all log lines without capping")
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("rendered view missing %q\n---\n%s\n---", needle, haystack)
	}
}
