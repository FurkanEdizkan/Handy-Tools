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
	case mascot.StateWorking:
		stateStyle = s.Accent
	case mascot.StateSuccess:
		stateStyle = s.OK
	case mascot.StateError:
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
// expanded-log view for whichever job has Expanded=true.
func queueView(s theme.Styles, jobs []Job, width int, focused int) string {
	if width < 24 {
		width = 24
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		s.Accent.Bold(true).Render("QUEUE"),
		"  ",
		s.Dim.Render(jobCounts(jobs, s)),
	)

	rows := []string{header, ""}
	for i, j := range jobs {
		rows = append(rows, renderJobRow(s, j, width-4, i == focused))
		if j.Expanded && len(j.Logs) > 0 {
			rows = append(rows, renderJobLogs(s, j))
		}
		if j.Err != "" && !j.Expanded {
			rows = append(rows, s.Err.Render("  × "+j.Err))
		}
	}
	return s.Card.Width(width).Render(strings.Join(rows, "\n"))
}

func jobCounts(jobs []Job, s theme.Styles) string {
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
	return fmt.Sprintf("%s · %s · %s · %d queued",
		s.Accent.Render(fmt.Sprintf("%d run", run)),
		s.OK.Render(fmt.Sprintf("%d done", done)),
		s.Err.Render(fmt.Sprintf("%d failed", fail)),
		queued,
	)
}

func renderJobRow(s theme.Styles, j Job, width int, focused bool) string {
	ico, pill := jobIconAndPill(s, j)

	caret := s.Dim.Render(" ")
	if len(j.Logs) > 0 {
		if j.Expanded {
			caret = s.Accent.Render("▾")
		} else {
			caret = s.Dim.Render("▸")
		}
	}

	label := j.Label
	if j.Kind != "" {
		label += s.Dim.Render(" · " + j.Kind)
	}
	time := s.Dim.Render(j.Time)
	if j.Status == JobRunning {
		mini := miniBar(s, j.Progress, 8)
		time = mini + " " + time
	}

	line := lipgloss.JoinHorizontal(lipgloss.Top,
		ico, " ",
		lipgloss.NewStyle().Width(intMax(10, width-30)).Render(label),
		" ", time,
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

func renderJobLogs(s theme.Styles, j Job) string {
	title := lipgloss.JoinHorizontal(lipgloss.Top,
		s.Accent.Bold(true).Render("STDERR · "+j.ID),
		"  ",
		s.Dim.Render(fmt.Sprintf("%d lines", len(j.Logs))),
	)
	body := []string{title}
	for _, l := range j.Logs {
		body = append(body, renderLogLine(s, l))
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

func renderLogLine(s theme.Styles, l LogLine) string {
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
	ts := s.Dim.Render("[" + l.T + "]")
	lvl := lvlStyle.Render(padRight(l.Lvl, 5))
	msg := msgStyle.Render(l.Msg)
	return lipgloss.JoinHorizontal(lipgloss.Top, ts, " ", lvl, " ", msg)
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
