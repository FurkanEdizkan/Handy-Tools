package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy-tools/internal/ui/mascot"
	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
)

// JobStatus mirrors the design's queue states. Strings match the lowercase
// labels rendered into the status pills (queued / running / done / failed).
type JobStatus string

const (
	JobQueued  JobStatus = "queued"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
)

// LogLine is one rendered stderr entry in a job's expandable log view.
type LogLine struct {
	T   string // pre-formatted timestamp ("14:08:22.034")
	Lvl string // INFO / DEBUG / WARN / ERROR / HINT / DONE / FAIL
	Msg string
}

// Job is one row in the queue panel.
type Job struct {
	ID       string
	Label    string
	Kind     string
	Status   JobStatus
	Time     string
	Progress int // 0..100, only meaningful while Running
	Err      string
	Logs     []LogLine
	Expanded bool
}

// stateBlock renders the left-column "STATE / CURRENT / progress" card.
func stateBlock(s theme.Styles, st mascot.State, taskLabel string, progress int, width int) string {
	stateName := strings.ToLower(st.String())

	stateStyle := s.Dim
	switch st {
	case mascot.StateThinking:
		stateStyle = s.Warn
	case mascot.StateWatching, mascot.StateStressed:
		stateStyle = s.Accent
	case mascot.StateTired:
		stateStyle = s.Dim
	case mascot.StateHappy:
		stateStyle = s.OK
	case mascot.StateWorried:
		stateStyle = s.Err
	}

	row := func(lbl, val string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			s.Section.Render(strings.ToUpper(lbl)),
			lipgloss.NewStyle().Width(2).Render(""),
			val,
		)
	}

	task := s.Dim.Render("— no active task —")
	if taskLabel != "" {
		task = lipgloss.NewStyle().Foreground(s.P.Text).Render(taskLabel)
	}

	bar := progressBar(s, progress, intMax(20, width-6))
	pct := s.Accent.Bold(true).Render(fmt.Sprintf("%3d%%", progress))
	meta := s.Dim.Render("waiting for input")
	if progress > 0 && progress < 100 {
		meta = s.Dim.Render("in flight")
	} else if progress == 100 {
		meta = s.OK.Render("complete")
	}

	gap := lipgloss.NewStyle().Width(intMax(1, width-len(stateName)-22)).Render("")
	body := lipgloss.JoinVertical(lipgloss.Left,
		row("state", stateStyle.Bold(true).Render(stateName)),
		row("current", task),
		"",
		bar,
		lipgloss.JoinHorizontal(lipgloss.Top, meta, gap, pct),
	)

	card := s.Card
	if width > 0 {
		card = card.Width(width)
	}
	return card.Render(body)
}

// progressBar renders an ASCII progress bar of the given width.
func progressBar(s theme.Styles, pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	if width < 4 {
		width = 4
	}
	fill := (width * pct) / 100
	return s.BarFill.Render(strings.Repeat("█", fill)) +
		s.BarTrack.Render(strings.Repeat("░", width-fill))
}

// queueView renders the left-column "Queue" card with all jobs + an
// expanded-log view for whichever job has Expanded=true. The expanded
// stderr block is capped at maxLogRows visual rows so the left column
// stays inside the terminal viewport on short windows. Pass 0 for an
// uncapped panel (used when height isn't known yet).
func queueView(s theme.Styles, jobs []Job, width int, focused int, maxLogRows int) string {
	if width < 24 {
		width = 24
	}
	// Reserve room for "QUEUE  " (the title + gap) when budgeting the
	// counts strip so the compact fallback engages at narrow widths
	// instead of letting the strip wrap onto a second line.
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		s.Accent.Bold(true).Render("QUEUE"),
		"  ",
		jobCounts(jobs, s, width-4-len("QUEUE  ")),
	)

	rows := []string{header, ""}
	contentWidth := width - 4
	for i, j := range jobs {
		rows = append(rows, renderJobRow(s, j, contentWidth, i == focused))
		if j.Expanded && len(j.Logs) > 0 {
			rows = append(rows, renderJobLogs(s, j, contentWidth, maxLogRows))
		}
		if j.Err != "" && !j.Expanded {
			rows = append(rows, s.Err.Render("  × "+j.Err))
		}
	}
	return s.Card.Width(width).Render(strings.Join(rows, "\n"))
}

// jobCounts emits the "0 run · 1 done · 1 failed · 2 queued" status
// strip the queue header carries. budget is the available cell width;
// when the design's long form ("1 failed", "2 queued") would wrap we
// fall back to a single-letter form ("1F 2Q") so the header stays one
// line. Pass 0 to disable the compact fallback.
func jobCounts(jobs []Job, s theme.Styles, budget int) string {
	var run, done, fail, queued int
	for _, j := range jobs {
		switch j.Status {
		case JobRunning:
			run++
		case JobDone:
			done++
		case JobFailed:
			fail++
		case JobQueued:
			queued++
		}
	}
	full := fmt.Sprintf("%s · %s · %s · %d queued",
		s.Accent.Render(fmt.Sprintf("%d run", run)),
		s.OK.Render(fmt.Sprintf("%d done", done)),
		s.Err.Render(fmt.Sprintf("%d failed", fail)),
		queued,
	)
	if budget <= 0 || lipgloss.Width(full) <= budget {
		return full
	}
	return fmt.Sprintf("%s %s %s %s",
		s.Accent.Render(fmt.Sprintf("%dR", run)),
		s.OK.Render(fmt.Sprintf("%dD", done)),
		s.Err.Render(fmt.Sprintf("%dF", fail)),
		s.Dim.Render(fmt.Sprintf("%dQ", queued)),
	)
}

func renderJobRow(s theme.Styles, j Job, width int, focused bool) string {
	ico, pill := jobIconAndPill(s, j)

	caret := " "
	if len(j.Logs) > 0 {
		if j.Expanded {
			caret = s.Accent.Render("▾")
		} else {
			caret = s.Dim.Render("▸")
		}
	}

	timeText := j.Time
	timeWidth := lipgloss.Width(timeText)
	timePiece := s.Dim.Render(timeText)
	if j.Status == JobRunning {
		mini := miniBar(s, j.Progress, 8)
		timePiece = mini + " " + timePiece
		timeWidth += 8 + 1
	}

	// Reserve the fixed cells: icon(1) + space + space + time + space +
	// pill + space + caret(1). Whatever's left is the label budget; the
	// label is truncated to that budget with a "…" tail rather than
	// being soft-wrapped by lipgloss (the design uses CSS
	// text-overflow: ellipsis, so we match that).
	overhead := 1 + 1 + 1 + timeWidth + 1 + lipgloss.Width(pill) + 1 + 1
	labelBudget := width - overhead
	if labelBudget < 6 {
		labelBudget = 6
	}

	plain := j.Label
	kindSuffix := ""
	if j.Kind != "" {
		kindSuffix = " · " + j.Kind
	}
	full := plain + kindSuffix
	runes := []rune(full)
	if len(runes) > labelBudget {
		full = string(runes[:labelBudget-1]) + "…"
		// Style the visible substring. If the truncation chopped into
		// the kind suffix, just dim the trailing "·" separator we can
		// still see; otherwise dim the whole " · kind" tail.
		if idx := strings.Index(full, " · "); idx >= 0 {
			plain = full[:idx]
			kindSuffix = full[idx:]
		} else {
			plain = full
			kindSuffix = ""
		}
	}
	label := plain
	if kindSuffix != "" {
		label += s.Dim.Render(kindSuffix)
	}
	labelCell := lipgloss.NewStyle().Width(labelBudget).MaxHeight(1).Render(label)

	line := lipgloss.JoinHorizontal(lipgloss.Top,
		ico, " ",
		labelCell,
		" ", timePiece,
		" ", pill,
		" ", caret,
	)

	if focused {
		line = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "", Dark: ""}).
			Foreground(s.P.Text).
			Render("▸ " + line)
	} else {
		line = "  " + line
	}
	return line
}

func jobIconAndPill(s theme.Styles, j Job) (string, string) {
	switch j.Status {
	case JobRunning:
		return s.Accent.Render("◐"), s.BadgeRun.Render("RUN")
	case JobDone:
		return s.OK.Render("✓"), s.BadgeOK.Render("DONE")
	case JobFailed:
		return s.Err.Render("✕"), s.BadgeErr.Render("FAIL")
	case JobQueued:
		return s.Dim.Render("•"), s.BadgeMute.Render("WAIT")
	}
	return "•", ""
}

func miniBar(s theme.Styles, pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	fill := (width * pct) / 100
	return s.BarFill.Render(strings.Repeat("▰", fill)) +
		s.BarTrack.Render(strings.Repeat("▱", width-fill))
}

// renderJobLogs renders the expanded stderr panel for a single job.
// contentWidth is the inner width of the queue card (caller subtracts
// the card border + padding). maxLogRows caps how many log entries we
// emit; 0 means no cap. When entries are dropped, we keep the most
// recent ones and prepend a "… +N earlier" marker so the failure tail
// (where the actionable HINT/FAIL lines live) stays visible.
func renderJobLogs(s theme.Styles, j Job, contentWidth, maxLogRows int) string {
	logs := j.Logs
	omitted := 0
	// renderJobLogs's box has Padding(0, 1) → 2 cells of horizontal
	// padding, so each log line gets contentWidth-2 cells.
	innerWidth := contentWidth - 2
	if innerWidth < 12 {
		innerWidth = 12
	}

	title := lipgloss.JoinHorizontal(lipgloss.Top,
		s.Accent.Bold(true).Render("STDERR · "+j.ID),
		"  ",
		s.Dim.Render(fmt.Sprintf("%d lines", len(j.Logs))),
	)

	// Reserve rows for the title and (when running) the streaming marker
	// so the cap counts the user-facing entries, not framing.
	overhead := 1
	if j.Status == JobRunning {
		overhead++
	}
	if maxLogRows > 0 && len(logs)+overhead > maxLogRows {
		// keep at least one log line, leave one row for the omission notice
		keep := maxLogRows - overhead - 1
		if keep < 1 {
			keep = 1
		}
		if keep < len(logs) {
			omitted = len(logs) - keep
			logs = logs[omitted:]
		}
	}

	body := []string{title}
	if omitted > 0 {
		body = append(body, s.Dim.Render(fmt.Sprintf("  … +%d earlier", omitted)))
	}
	for _, l := range logs {
		body = append(body, renderLogLine(s, l, innerWidth))
	}
	if j.Status == JobRunning {
		body = append(body, s.Dim.Render("[…] …   streaming…"))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(s.P.Border).
		Padding(0, 1).
		Foreground(s.P.Text).
		Render(strings.Join(body, "\n"))
	return box
}

// renderLogLine renders one stderr entry. maxWidth is the available
// display width inside the expanded-log box; the timestamp prefix is
// dropped first when room is tight, and the message tail is ellipsized
// before the line ever has to soft-wrap. Pass 0 for the legacy
// uncapped layout.
func renderLogLine(s theme.Styles, l LogLine, maxWidth int) string {
	lvlStyle := s.Dim
	msgStyle := lipgloss.NewStyle().Foreground(s.P.Text)
	switch l.Lvl {
	case "DEBUG":
		lvlStyle, msgStyle = s.Dim, s.Dim
	case "WARN":
		lvlStyle, msgStyle = s.Warn, s.Warn
	case "ERROR", "FAIL":
		lvlStyle, msgStyle = s.Err.Bold(true), s.Err
	case "DONE":
		lvlStyle, msgStyle = s.OK.Bold(true), s.OK
	case "HINT":
		lvlStyle, msgStyle = s.Accent.Bold(true), s.Accent
	case "INFO":
		lvlStyle = lipgloss.NewStyle().Foreground(s.P.Text).Bold(true)
	}
	tsPlain := "[" + l.T + "]"
	lvlPlain := padRight(l.Lvl, 5)
	tsWidth := lipgloss.Width(tsPlain) + 1 // trailing space after [ts]
	lvlWidth := lipgloss.Width(lvlPlain) + 1
	showTs := true
	msg := l.Msg
	if maxWidth > 0 {
		budget := maxWidth - tsWidth - lvlWidth
		// Narrow column — drop the timestamp to give the message room.
		if budget < 12 && maxWidth-lvlWidth >= 12 {
			showTs = false
			budget = maxWidth - lvlWidth
		}
		if budget < 4 {
			budget = 4
		}
		runes := []rune(msg)
		if len(runes) > budget {
			if budget > 1 {
				msg = string(runes[:budget-1]) + "…"
			} else {
				msg = "…"
			}
		}
	}
	parts := []string{}
	if showTs {
		parts = append(parts, s.Dim.Render(tsPlain), " ")
	}
	parts = append(parts,
		lvlStyle.Render(lvlPlain), " ",
		msgStyle.Render(msg),
	)
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
