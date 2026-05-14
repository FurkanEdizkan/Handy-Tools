// Package ui hosts the Bubble Tea models. The top-level Router presents the
// design's two-pane layout: the mascot, state block, and queue panel live in
// a fixed left column; the right column swaps between the Home tool catalog
// and a per-tool detail page. Settings still exist as a sub-page reachable
// from the header.
//
// Pages don't hold their own mascot — instead they send MascotMsg / RunJob /
// OpenTool / GoHome through the router, which mutates the shared state.
package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/ui/mascot"
	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
)

// PageID identifies the right-column page.
type PageID int

const (
	PageHome PageID = iota
	PageTool
	PageSettings
)

// Page is the contract every right-column screen implements.
type Page interface {
	Init() tea.Cmd
	Update(tea.Msg) (Page, tea.Cmd)
	View() string
	ID() PageID
	Title() string
}

// ---------- Cross-page messages ----------

// Navigate switches the right-column page.
type Navigate struct{ To PageID }

// OpenTool is emitted by the home page when the user picks a tool.
type OpenTool struct{ ID string }

// GoHome is emitted by the tool page (Esc).
type GoHome struct{}

// RunJob is emitted by the tool page when the user hits Run. The router
// enqueues a simulated job into the queue panel and steps it forward.
type RunJob struct {
	Tool        tool
	Files       []fileItem
	Summary     string
	ArchiveMode archiveMode
	ArchiveOut  string
	PDFOp       pdfOp
	Out         outDest
	CustomPath  string
}

// MascotMsg lets pages drive the global mascot.
type MascotMsg struct {
	State    mascot.State
	Greeting string
	Speech   string
}

// runStepMsg advances the active simulated job by one tick.
type runStepMsg struct{ jobID string }

// Model is the top-level Bubble Tea program.
type Model struct {
	cfg     config.Config
	styles  theme.Styles
	mascot  mascot.Model
	pages   map[PageID]Page
	current PageID

	tool *toolPage // active tool page (nil when current != PageTool)

	queue       []Job
	progress    int
	currentTask string
	running     bool
	runJobID    string
	toast       string
	toastUntil  time.Time

	width    int
	height   int
	quitting bool
}

// New builds a router seeded with the given config.
func New(cfg config.Config) Model {
	styles := theme.Build(theme.Resolve(cfg.Theme.Name))
	m := Model{
		cfg:     cfg,
		styles:  styles,
		mascot:  mascot.New(styles),
		pages:   map[PageID]Page{},
		current: PageHome,
		queue:   seedQueue(),
	}
	m.pages[PageHome] = newHomePage(styles)
	m.pages[PageSettings] = newSettingsPage(styles, &m.cfg)
	m.mascot.Say("Hi! I'm Wrenly.")
	m.mascot.Whisper(speechFor("convert-image"))
	return m
}

// seedQueue mirrors the JSX INITIAL_QUEUE so the queue panel reads as
// populated (one running, one done, one failed-with-logs).
func seedQueue() []Job {
	return []Job{
		{
			ID: "q1", Label: "invoice-2026-04.png → JPEG", Kind: "image",
			Status: JobDone, Time: "14:02",
			Logs: []LogLine{
				{T: "14:02:01.012", Lvl: "INFO", Msg: "image.convert: starting · 1 file"},
				{T: "14:02:01.018", Lvl: "DEBUG", Msg: "image.decode: invoice-2026-04.png · PNG 1840×2400 · 1.21 MB"},
				{T: "14:02:01.214", Lvl: "INFO", Msg: "image.encode: jpeg quality=90 progressive=false"},
				{T: "14:02:01.412", Lvl: "INFO", Msg: "image.write: ./out/invoice-2026-04.jpg · 412 KB (66% smaller)"},
				{T: "14:02:01.413", Lvl: "DONE", Msg: "1 file processed in 401 ms"},
			},
		},
		{
			ID: "q3", Label: "manual.pdf → 32 pages", Kind: "pdf",
			Status: JobFailed, Time: "14:08",
			Err:      "pdftoppm not found · brew install poppler",
			Expanded: true,
			Logs: []LogLine{
				{T: "14:08:22.004", Lvl: "INFO", Msg: "pdf.render: opening manual.pdf"},
				{T: "14:08:22.018", Lvl: "DEBUG", Msg: "pdf.inspect: 32 pages · 8.4 MB · PDF 1.7"},
				{T: "14:08:22.020", Lvl: "INFO", Msg: "render plan: 32 pages → PNG @ 150 dpi → ./out/manual/"},
				{T: "14:08:22.022", Lvl: "DEBUG", Msg: "sysdep.find: looking for \"pdftoppm\" on $PATH"},
				{T: "14:08:22.031", Lvl: "WARN", Msg: "sysdep.find: tried /usr/local/bin, /usr/bin, /opt/homebrew/bin"},
				{T: "14:08:22.032", Lvl: "ERROR", Msg: "sysdep.MISSING_BINARY: pdftoppm not present"},
				{T: "14:08:22.033", Lvl: "ERROR", Msg: "tools/pdf.render: cannot render without pdftoppm (code MISSING_BINARY)"},
				{T: "14:08:22.034", Lvl: "HINT", Msg: "brew install poppler  # macOS"},
				{T: "14:08:22.034", Lvl: "HINT", Msg: "apt install poppler-utils  # Debian / Ubuntu"},
				{T: "14:08:22.035", Lvl: "FAIL", Msg: "job aborted after 318 ms · 0 pages rendered"},
			},
		},
		{ID: "q4", Label: "photos.zip → ./photos/", Kind: "archive", Status: JobQueued, Time: "—"},
		{ID: "q5", Label: "big-batch (24 PNGs) → WebP", Kind: "image", Status: JobQueued, Time: "—"},
	}
}

// Init begins the mascot animation loop and any per-page init commands.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{mascot.Tick()}
	for _, p := range m.pages {
		if c := p.Init(); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

// Update routes messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		left := intMax(28, m.width/4)
		if left > 42 {
			left = 42
		}
		m.mascot.SetWidth(left - 2)
		if hp, ok := m.pages[PageHome].(*homePage); ok {
			hp.SetWidth(m.width - left - 6)
		}
		if m.tool != nil {
			m.tool.SetWidth(m.width - left - 6)
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			if m.current == PageHome {
				// theme cycling matches the design's Tab shortcut on home
				order := []string{"forge", "snow", "ember"}
				i := 0
				for j, n := range order {
					if n == m.cfg.Theme.Name {
						i = j
					}
				}
				next := order[(i+1)%len(order)]
				m.cfg.Theme.Name = next
				m.styles = theme.Build(theme.Resolve(next))
				m.mascot.SetStyles(m.styles)
				// rebuild pages with new styles
				m.pages[PageHome] = newHomePage(m.styles)
				m.pages[PageSettings] = newSettingsPage(m.styles, &m.cfg)
				m.showToast("theme · " + next)
				return m, nil
			}
		case ",":
			if m.current == PageSettings {
				m.current = PageHome
			} else {
				m.current = PageSettings
			}
			return m, nil
		case "esc":
			if m.current == PageTool && !m.running {
				m.current = PageHome
				m.tool = nil
				m.mascot.Set(mascot.StateIdle)
				m.mascot.Whisper("Pick a tool to get started.")
				return m, nil
			}
			if m.current == PageSettings {
				m.current = PageHome
				return m, nil
			}
		}
	case Navigate:
		m.current = msg.To
		return m, nil
	case OpenTool:
		t, ok := lookupTool(msg.ID)
		if !ok {
			return m, nil
		}
		m.tool = newToolPage(m.styles, t)
		left := intMax(28, m.width/4)
		if left > 42 {
			left = 42
		}
		m.tool.SetWidth(m.width - left - 6)
		m.current = PageTool
		m.mascot.Set(mascot.StateThinking)
		m.mascot.Whisper("Drop in your files and pick an output format.")
		return m, nil
	case GoHome:
		m.current = PageHome
		m.tool = nil
		m.mascot.Set(mascot.StateIdle)
		m.mascot.Whisper("Pick a tool to get started.")
		return m, nil
	case RunJob:
		return m, m.startRun(msg)
	case runStepMsg:
		return m.stepRun(msg.jobID)
	case MascotMsg:
		m.mascot.Set(msg.State)
		if msg.Greeting != "" {
			m.mascot.Say(msg.Greeting)
		}
		if msg.Speech != "" {
			m.mascot.Whisper(msg.Speech)
		}
		return m, nil
	}

	// forward animation ticks to the mascot
	mm, mc := m.mascot.Update(msg)
	m.mascot = mm

	// forward keys to the active right-column page
	var pc tea.Cmd
	switch m.current {
	case PageTool:
		if m.tool != nil {
			tp, c := m.tool.Update(msg)
			m.tool = tp
			pc = c
		}
	default:
		page := m.pages[m.current]
		updated, c := page.Update(msg)
		m.pages[m.current] = updated
		pc = c
	}
	return m, tea.Batch(mc, pc)
}

// startRun pushes a "running" job into the queue and schedules the first
// progress step. Mirrors the JSX startRun simulation.
func (m *Model) startRun(j RunJob) tea.Cmd {
	if m.running {
		return nil
	}
	id := fmt.Sprintf("r%d", time.Now().UnixNano()%1000000)
	now := time.Now()
	hh, mm, _ := now.Clock()
	timeStr := fmt.Sprintf("%02d:%02d", hh, mm)
	taskLabel := fmt.Sprintf("%s · %s", j.Tool.label, j.Summary)
	startupLogs := []LogLine{
		{T: ts(now, 0), Lvl: "INFO", Msg: fmt.Sprintf("%s: queued %d input(s)", j.Tool.id, len(j.Files))},
		{T: ts(now, 6), Lvl: "DEBUG", Msg: "output → " + outPath(j)},
		{T: ts(now, 12), Lvl: "INFO", Msg: "opening worker pool · 4 threads"},
	}
	job := Job{
		ID: id, Label: taskLabel, Kind: j.Tool.id,
		Status: JobRunning, Time: timeStr, Progress: 0, Logs: startupLogs,
	}
	m.queue = append([]Job{job}, m.queue...)
	m.running = true
	m.runJobID = id
	m.currentTask = fmt.Sprintf("%s · 0/%d", j.Tool.label, len(j.Files))
	m.progress = 0
	m.mascot.Set(mascot.StateThinking)
	m.mascot.Whisper("Working on " + j.Tool.label + "…")
	return tea.Tick(450*time.Millisecond, func(time.Time) tea.Msg {
		return runStepMsg{jobID: id}
	})
}

// stepRun advances the running job by ~12% per tick and emits a per-file
// "processed" log line. Completes by flipping the job to Done + the mascot
// to Success.
func (m Model) stepRun(jobID string) (tea.Model, tea.Cmd) {
	idx := -1
	for i, j := range m.queue {
		if j.ID == jobID {
			idx = i
			break
		}
	}
	if idx < 0 {
		m.running = false
		return m, nil
	}
	job := m.queue[idx]
	now := time.Now()
	step := 12
	job.Progress += step
	if job.Progress > 100 {
		job.Progress = 100
	}

	// determine if we've crossed a file boundary
	files := 1
	if m.tool != nil {
		files = len(m.tool.files)
		if files == 0 {
			files = 1
		}
	}
	done := (job.Progress * files) / 100
	prev := ((job.Progress - step) * files) / 100
	for k := prev; k < done && k < files; k++ {
		name := fmt.Sprintf("item-%d", k+1)
		if m.tool != nil && k < len(m.tool.files) {
			name = m.tool.files[k].Name
		}
		job.Logs = append(job.Logs, LogLine{
			T:   ts(now, 0),
			Lvl: "INFO",
			Msg: "processed " + name,
		})
	}

	if job.Progress >= 100 {
		job.Status = JobDone
		job.Progress = 100
		job.Logs = append(job.Logs, LogLine{
			T: ts(now, 0), Lvl: "DONE",
			Msg: fmt.Sprintf("%d items processed", files),
		})
		m.queue[idx] = job
		m.running = false
		m.currentTask = ""
		m.progress = 100
		m.mascot.Set(mascot.StateSuccess)
		m.showToast(fmt.Sprintf("done · %d files processed", files))
		return m, tea.Tick(1800*time.Millisecond, func(time.Time) tea.Msg {
			return MascotMsg{State: mascot.StateIdle, Speech: speechFor(m.activeToolID())}
		})
	}

	m.queue[idx] = job
	m.progress = job.Progress
	m.currentTask = fmt.Sprintf("%s · %d/%d", m.tool.tool.label, done, files)
	if job.Progress > 30 {
		m.mascot.Set(mascot.StateWorking)
	}
	return m, tea.Tick(450*time.Millisecond, func(time.Time) tea.Msg {
		return runStepMsg{jobID: jobID}
	})
}

func (m Model) activeToolID() string {
	if m.tool != nil {
		return m.tool.tool.id
	}
	return ""
}

// outPath renders the chosen output destination for the startup log line.
func outPath(j RunJob) string {
	switch j.Out {
	case outAlongside:
		return "alongside input"
	case outCustom:
		return j.CustomPath
	}
	return "./out"
}

// ts builds a "HH:MM:SS.mmm" timestamp offset by addMS milliseconds.
func ts(now time.Time, addMS int) string {
	t := now.Add(time.Duration(addMS) * time.Millisecond)
	return fmt.Sprintf("%02d:%02d:%02d.%03d",
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1e6)
}

// lookupTool finds a tool by id.
func lookupTool(id string) (tool, bool) {
	for _, t := range defaultTools {
		if t.id == id {
			return t, true
		}
	}
	return tool{}, false
}

// showToast schedules a transient floating message under the screen.
func (m *Model) showToast(text string) {
	m.toast = text
	m.toastUntil = time.Now().Add(2200 * time.Millisecond)
}

// View composes the whole TUI: titlebar → header → left/right split → status.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	left := intMax(28, m.width/4)
	if left > 42 {
		left = 42
	}
	rightWidth := intMax(40, m.width-left-6)

	// header strip
	cogLabel := " ⚙ settings "
	cog := m.styles.IconBtn.Render(cogLabel)
	if m.current == PageSettings {
		cog = m.styles.IconBtn.Foreground(m.styles.P.Accent).BorderForeground(m.styles.P.Accent).Render(cogLabel)
	}
	brand := m.styles.Brand.Render("◤ HANDY TOOLS") + m.styles.Sub.Render(" / htools")
	crumbTitle := "Home"
	if m.current == PageTool && m.tool != nil {
		crumbTitle = m.tool.tool.label
	} else if m.current == PageSettings {
		crumbTitle = "Settings"
	}
	crumb := m.styles.Crumb.Render(" › ") + lipgloss.NewStyle().Foreground(m.styles.P.Text).Bold(true).Render(crumbTitle)
	version := m.styles.Sub.Render("  v" + buildVer())
	conn := m.styles.OK.Render("●")
	if m.running {
		conn = m.styles.Accent.Render("●")
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, cog, "  ", brand, crumb,
		lipgloss.NewStyle().Width(intMax(2, m.width-lipgloss.Width(cog)-lipgloss.Width(brand)-lipgloss.Width(crumb)-12)).Render(""),
		version, " ", conn)

	// left column
	leftCol := lipgloss.JoinVertical(lipgloss.Left,
		m.mascot.View(),
		stateBlock(m.styles, m.mascot.State(), m.currentTask, m.progress, left),
		queueView(m.styles, m.queue, left, -1),
	)

	// right column
	var rightCol string
	switch m.current {
	case PageHome:
		rightCol = m.pages[PageHome].View()
	case PageTool:
		if m.tool != nil {
			rightCol = m.tool.View()
		}
	case PageSettings:
		rightCol = m.pages[PageSettings].View()
	}
	rightCol = lipgloss.NewStyle().Width(rightWidth).Render(rightCol)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)
	status := m.renderStatusBar()

	frame := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", status)
	if m.toast != "" && time.Now().Before(m.toastUntil) {
		frame += "\n" + m.styles.Toast.Render(m.toast)
	}
	return m.styles.App.Render(frame)
}

// renderStatusBar produces the bottom keyboard-hint strip.
func (m Model) renderStatusBar() string {
	s := m.styles
	chip := func(k, label string) string {
		return s.KbdChip.Render(k) + " " + s.Dim.Render(label)
	}
	parts := []string{}
	switch m.current {
	case PageHome:
		parts = append(parts,
			chip("↑↓", "move"),
			chip("↵", "open"),
			chip("1-5", "jump"),
		)
	case PageTool:
		parts = append(parts,
			chip("esc", "back"),
			chip("↑↓", "focus"),
			chip("space", "toggle"),
			chip("r", "run"),
		)
	case PageSettings:
		parts = append(parts,
			chip("esc", "back"),
			chip("↵", "cycle"),
			chip("s", "save"),
		)
	}
	parts = append(parts,
		chip("tab", "theme"),
		chip(",", "settings"),
		chip("q", "quit"),
	)
	right := s.Dim.Render("idle · htoolsd offline")
	if m.running {
		right = s.Accent.Render("busy · 1 running") + s.Dim.Render(" · htoolsd offline")
	}
	hints := ""
	for i, p := range parts {
		if i > 0 {
			hints += "   "
		}
		hints += p
	}
	pad := m.width - lipgloss.Width(hints) - lipgloss.Width(right) - 2
	if pad < 1 {
		pad = 1
	}
	return hints + " " + lipgloss.NewStyle().Width(pad).Render("") + right
}

// buildVer returns a short version string for the header — keeps the design's
// "v0.1.0" stamp visible without pulling in buildinfo here.
func buildVer() string {
	return "0.1.0"
}
