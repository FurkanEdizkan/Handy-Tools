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
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy-tools/internal/config"
	jobqueue "github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/ui/mascot"
	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
)

// PageID identifies the right-column page.
type PageID int

const (
	PageHome PageID = iota
	PageTool
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

// OpenTool is emitted by the home page when the user picks a tool.
type OpenTool struct{ ID string }

// GoHome is emitted by the tool page (Esc).
type GoHome struct{}

// RunJob is emitted by the tool page when the user hits Run. The router
// enqueues a job onto the shared internal/queue; progress flows back as
// queueEventMsg via the SubscribeAll bridge.
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

// queueEventMsg carries one job-state snapshot drained from the shared
// queue's SubscribeAll stream into the Bubble Tea event loop.
type queueEventMsg struct{ job jobqueue.Job }

// Model is the top-level Bubble Tea program.
type Model struct {
	cfg     config.Config
	styles  theme.Styles
	mascot  mascot.Model
	pages   map[PageID]Page
	current PageID

	tool *toolPage // active tool page (nil when current != PageTool)

	pop settingsPopover // overlay settings popover (open by `,` or cog)

	// jobQueue is the shared job registry; events is its SubscribeAll
	// stream, drained into the Bubble Tea loop by waitForQueueEvent.
	jobQueue *jobqueue.Queue
	events   <-chan jobqueue.Job

	queue       []Job // display rows, folded from queue snapshots
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

// New builds a router seeded with the given config and the shared job queue.
// q may be nil — the queue panel then stays empty and the Run button is a
// no-op — but cmd/htools always passes a real queue.
func New(cfg config.Config, q *jobqueue.Queue) Model {
	styles := theme.Build(theme.Resolve(cfg.Theme.Name))
	m := Model{
		cfg:      cfg,
		styles:   styles,
		mascot:   mascot.New(styles),
		pages:    map[PageID]Page{},
		current:  PageHome,
		jobQueue: q,
	}
	if q != nil {
		// A process-lifetime subscription: the TUI runs until the process
		// exits, so the Background context never needs canceling.
		m.events = q.SubscribeAll(context.Background())
	}
	m.mascot.SetCharacter(characterFromConfig(cfg.Mascot.Style))
	m.pages[PageHome] = newHomePage(styles)
	m.pop = newSettingsPopover()
	// Greeting mirrors the design's "Hi! I'm Wrenly." line; the
	// character-specific name is swapped in via mascot.Say so the
	// label changes when the user picks Hopper.
	m.mascot.Say(greetingFor(cfg.Mascot.Style))
	m.mascot.Whisper(speechFor("convert-image"))
	m.applyIdleMood()
	return m
}

// mascotHeightBudget translates the total terminal height into the
// number of rows the mascot card may use without pushing the rest of
// the left column past the viewport. The fixed overhead — header,
// status bar, frame spacers, state card (~7), queue card (~12) — eats
// roughly 26 rows. When the remainder is below the full 22-row sprite
// layout, the mascot renders compactly (badge + greeting + speech)
// instead.
func mascotHeightBudget(termHeight int) int {
	if termHeight <= 0 {
		return 0
	}
	budget := termHeight - 26
	if budget < 4 {
		budget = 4
	}
	return budget
}

// leftColWidth returns the fixed-sidebar width for a given terminal
// width. The 15-cell Wrenly sprite plus the mascot card's 2-cell border
// and 2-cell padding needs at least 34 columns, otherwise lipgloss
// soft-wraps each sprite row and the mascot reads as garbled dots. The
// design's CSS pins the sidebar to 340px (about 42 monospace cells) so
// we also cap at 42 to keep the right column from collapsing on
// ultra-wide terminals.
func leftColWidth(termWidth int) int {
	w := termWidth / 4
	if w < 34 {
		w = 34
	}
	if w > 42 {
		w = 42
	}
	return w
}

// characterFromConfig maps the on-disk cfg.Mascot.Style string onto the
// mascot's character key. The legacy default value "classic" is treated as
// Wrenly so existing configs keep working.
func characterFromConfig(style string) string {
	switch style {
	case mascot.CharacterHopper:
		return mascot.CharacterHopper
	default:
		return mascot.CharacterWrenly
	}
}

// greetingFor returns the mascot's name-stamped opening line — mirrors
// the design's "Hi! I'm Wrenly." (or "Hopper.") title-cased greeting.
func greetingFor(style string) string {
	switch characterFromConfig(style) {
	case mascot.CharacterHopper:
		return "Hi! I'm Hopper."
	default:
		return "Hi! I'm Wrenly."
	}
}

// Init begins the mascot animation loop, the queue-event bridge, and any
// per-page init commands.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{mascot.Tick()}
	if m.events != nil {
		cmds = append(cmds, waitForQueueEvent(m.events))
	}
	for _, p := range m.pages {
		if c := p.Init(); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

// waitForQueueEvent blocks on the next job snapshot from the shared queue and
// delivers it as a queueEventMsg. The queueEventMsg handler re-arms it, so one
// snapshot is drained per Update cycle — the idiomatic Bubble Tea bridge for a
// long-lived channel (no *tea.Program handle required).
func waitForQueueEvent(ch <-chan jobqueue.Job) tea.Cmd {
	return func() tea.Msg {
		j, ok := <-ch
		if !ok {
			return nil
		}
		return queueEventMsg{job: j}
	}
}

// Update routes messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		left := leftColWidth(m.width)
		m.mascot.SetWidth(left - 2)
		// Give the mascot a height hint so it can drop the 14-row sprite
		// and render compact when the leftcol must share the viewport
		// with the state + queue cards on short terminals.
		m.mascot.SetMaxHeight(mascotHeightBudget(m.height))
		if hp, ok := m.pages[PageHome].(*homePage); ok {
			hp.SetWidth(m.width - left - 6)
		}
		if m.tool != nil {
			m.tool.SetWidth(m.width - left - 6)
		}
		return m, nil
	case tea.KeyMsg:
		// The settings popover swallows keys while it's open so the user can
		// drive theme/mascot/scanlines/htoolsd chips without the home menu
		// stealing them.
		if m.pop.IsOpen() {
			return m.updatePopoverKey(msg)
		}
		// Likewise, a tool page capturing a typed path swallows every key —
		// otherwise 'q' would quit and ',' would open settings mid-path.
		if m.current == PageTool && m.tool != nil && m.tool.capturingText() {
			tp, c := m.tool.Update(msg)
			m.tool = tp
			return m, c
		}
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
				m.rebuildStyles()
				m.showToast("theme · " + next)
				// Persist so the choice survives the next launch. A
				// failed save overrides the toast but never aborts.
				m.persistConfig()
				return m, nil
			}
		case ",":
			m.pop.Toggle()
			return m, nil
		case "esc":
			if m.current == PageTool && !m.running {
				m.current = PageHome
				m.tool = nil
				m.applyIdleMood()
				return m, nil
			}
		}
	case OpenTool:
		t, ok := lookupTool(msg.ID)
		if !ok {
			return m, nil
		}
		m.tool = newToolPage(m.styles, t)
		left := leftColWidth(m.width)
		m.tool.SetWidth(m.width - left - 6)
		m.current = PageTool
		m.mascot.Set(mascot.StateThinking)
		m.mascot.Whisper(speechForState(mascot.StateThinking, t.id, t.label, 0))
		return m, nil
	case GoHome:
		m.current = PageHome
		m.tool = nil
		m.applyIdleMood()
		return m, nil
	case RunJob:
		m.startRun(msg)
		return m, nil
	case queueEventMsg:
		cmd := m.applyQueueEvent(msg.job)
		return m, tea.Batch(cmd, waitForQueueEvent(m.events))
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

// updatePopoverKey is the popover-specific keyboard handler. It runs first
// when the overlay is open so home/tool keys don't fire underneath.
func (m Model) updatePopoverKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", ",":
		m.pop.Close()
		return m, nil
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		m.pop.MoveUp()
		return m, nil
	case "down", "j":
		m.pop.MoveDown()
		return m, nil
	case "enter", " ", "right", "l":
		if m.pop.Cycle(&m.cfg) {
			m.rebuildStyles()
			m.syncMascotCharacter()
		}
		return m, nil
	}
	return m, nil
}

// persistConfig writes the current config to disk so settings such as the
// Tab-cycled theme survive a restart. A failure (e.g. a read-only config
// file) is surfaced as a toast — the in-memory choice still applies for the
// rest of this session, and the TUI never aborts over it.
func (m *Model) persistConfig() {
	if _, err := config.Save(m.cfg); err != nil {
		m.showToast("couldn't save settings: " + err.Error())
	}
}

// rebuildStyles refreshes the cached lipgloss styles after a theme change
// and re-seeds the pages that own their own styled state. Called from both
// the Tab cycle and the popover.
func (m *Model) rebuildStyles() {
	m.styles = theme.Build(theme.Resolve(m.cfg.Theme.Name))
	m.mascot.SetStyles(m.styles)
	m.pages[PageHome] = newHomePage(m.styles)
	if m.tool != nil {
		m.tool.styles = m.styles
	}
}

// startRun enqueues a job onto the shared queue. Progress flows back through
// the SubscribeAll bridge as queueEventMsg; an optimistic display row is
// prepended now so the queue panel reacts before the first event lands.
func (m *Model) startRun(j RunJob) {
	if m.jobQueue == nil || m.running {
		return
	}
	files := append([]fileItem(nil), j.Files...)
	id := m.jobQueue.Enqueue(j.Tool.id, "run", simulatedRunner(files))
	now := time.Now()
	hh, mn, _ := now.Clock()
	m.queue = append([]Job{{
		ID:     id,
		Label:  fmt.Sprintf("%s · %s", j.Tool.label, j.Summary),
		Kind:   j.Tool.id,
		Status: JobQueued,
		Time:   fmt.Sprintf("%02d:%02d", hh, mn),
	}}, m.queue...)
	m.running = true
	m.runJobID = id
	m.currentTask = fmt.Sprintf("%s · 0/%d", j.Tool.label, len(files))
	m.progress = 0
	m.mascot.Set(mascot.StateThinking)
	m.mascot.Whisper(speechForState(mascot.StateThinking, j.Tool.id, j.Tool.label, 0))
}

// simulatedRunner is the #77a placeholder Runner — it emits one progress event
// per input file, then returns so the queue marks the job complete. It proves
// the queue↔Bubble Tea plumbing end to end; #77b swaps it for real tool
// execution through the shared internal/server handlers.
func simulatedRunner(files []fileItem) jobqueue.Runner {
	n := len(files)
	if n == 0 {
		n = 1
	}
	return func(ctx context.Context, emit func(tools.Progress)) {
		for i := 1; i <= n; i++ {
			select {
			case <-ctx.Done():
				return
			case <-time.After(220 * time.Millisecond):
			}
			name := fmt.Sprintf("item-%d", i)
			if i-1 < len(files) {
				name = files[i-1].Name
			}
			emit(tools.Progress{
				Level:       tools.SeverityInfo,
				Fraction:    float64(i) / float64(n),
				CurrentItem: name,
				Message:     "processed " + name,
			})
		}
		emit(tools.Progress{
			Level:    tools.SeverityInfo,
			Fraction: 1,
			Message:  fmt.Sprintf("%d item(s) processed", n),
		})
	}
}

// applyQueueEvent folds one queue snapshot into the display queue. When the
// snapshot is for the job this TUI started it also drives the state block and
// the mascot mood, returning the post-success idle-return tick if any.
func (m *Model) applyQueueEvent(qj jobqueue.Job) tea.Cmd {
	idx := -1
	for i := range m.queue {
		if m.queue[i].ID == qj.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		// A job enqueued outside this TUI — the queue is shared, so still
		// surface it. Prepend a fresh row.
		m.queue = append([]Job{{ID: qj.ID}}, m.queue...)
		idx = 0
	}
	job := m.queue[idx]
	if job.Label == "" {
		job.Label = queueJobLabel(qj)
	}
	if job.Kind == "" {
		job.Kind = qj.Tool
	}
	job.Status = queueJobStatus(qj)
	job.Progress = clampPct(int(qj.Progress.Fraction * 100))
	if !qj.StartedAt.IsZero() {
		hh, mn, _ := qj.StartedAt.Clock()
		job.Time = fmt.Sprintf("%02d:%02d", hh, mn)
	}
	if msg := qj.Progress.Message; msg != "" {
		if n := len(job.Logs); n == 0 || job.Logs[n-1].Msg != msg {
			job.Logs = append(job.Logs, LogLine{
				T: ts(time.Now(), 0), Lvl: progressLevel(qj.Progress), Msg: msg,
			})
		}
	}
	if qj.Err != nil {
		job.Err = qj.Err.Message
		if qj.Err.Detail != "" {
			job.Err += " · " + qj.Err.Detail
		}
		job.Expanded = true
	}
	m.queue[idx] = job

	if qj.ID == m.runJobID {
		return m.syncMascotToJob(job)
	}
	return nil
}

// syncMascotToJob mirrors the active job's status onto the state block and the
// mascot. Mood while running follows the design's load curve.
func (m *Model) syncMascotToJob(job Job) tea.Cmd {
	switch job.Status {
	case JobDone:
		m.running = false
		m.currentTask = ""
		m.progress = 100
		m.mascot.Set(mascot.StateHappy)
		m.mascot.Whisper(speechForState(mascot.StateHappy, m.activeToolID(), "", 100))
		m.showToast("done · " + job.Label)
		// After basking in success briefly, drop back to the resting mood
		// (worried if a failed job still sits on the queue).
		st, sp := m.idleMood()
		return tea.Tick(1800*time.Millisecond, func(time.Time) tea.Msg {
			return MascotMsg{State: st, Speech: sp}
		})
	case JobFailed:
		m.running = false
		m.currentTask = ""
		m.mascot.Set(mascot.StateWorried)
		m.mascot.Whisper(speechForState(mascot.StateWorried, m.activeToolID(), "", 0))
		m.showToast("failed · " + job.Err)
		return nil
	default:
		m.running = true
		m.progress = job.Progress
		m.currentTask = job.Label
		mood := loadMood(job.Progress)
		if m.mascot.State() != mood {
			m.mascot.Set(mood)
			m.mascot.Whisper(speechForState(mood, m.activeToolID(), job.Label, job.Progress))
		}
		return nil
	}
}

// loadMood maps a running job's percentage onto the design's app.jsx load
// curve: <38% watching, 38-78% stressed, ≥78% tired.
func loadMood(pct int) mascot.State {
	switch {
	case pct < 38:
		return mascot.StateWatching
	case pct < 78:
		return mascot.StateStressed
	default:
		return mascot.StateTired
	}
}

// queueJobStatus derives a display status from a queue snapshot. A job with no
// progress emitted yet is queued — the queue's emit callback always stamps
// Progress.Tool, so an empty Tool means the Runner hasn't reported yet.
func queueJobStatus(qj jobqueue.Job) JobStatus {
	switch {
	case qj.Err != nil:
		return JobFailed
	case qj.Completed:
		return JobDone
	case qj.Progress.Tool == "":
		return JobQueued
	default:
		return JobRunning
	}
}

// queueJobLabel builds a display label for a job not started from this TUI
// (so it carries no RunJob-supplied summary).
func queueJobLabel(qj jobqueue.Job) string {
	label := qj.Tool
	if qj.Action != "" {
		label += " · " + qj.Action
	}
	if qj.Progress.CurrentItem != "" {
		label += " · " + qj.Progress.CurrentItem
	}
	return label
}

// progressLevel renders a Progress's severity as a queue-log level label.
func progressLevel(p tools.Progress) string {
	if p.Err != nil {
		return "ERROR"
	}
	switch p.Level {
	case tools.SeverityWarning:
		return "WARN"
	case tools.SeverityError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// clampPct bounds a percentage to 0..100.
func clampPct(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// idleMood picks the resting mascot state when no job is active. If any
// failed job still sits on the queue, the mascot drops into worried instead
// of idle — the design's "carry that concern until the user clears or
// retries it" rule.
func (m Model) idleMood() (mascot.State, string) {
	for _, j := range m.queue {
		if j.Status == JobFailed {
			return mascot.StateWorried, speechForState(mascot.StateWorried, m.activeToolID(), "", 0)
		}
	}
	return mascot.StateIdle, speechFor(m.activeToolID())
}

// applyIdleMood writes the resting mood + speech onto the mascot now. Used
// when navigating home, escaping a tool page, or finishing a run.
func (m *Model) applyIdleMood() {
	state, speech := m.idleMood()
	m.mascot.Set(state)
	m.mascot.Whisper(speech)
}

// syncMascotCharacter re-reads cfg.Mascot.Style and updates the mascot
// character if the user changed it in the Settings page.
func (m *Model) syncMascotCharacter() {
	m.mascot.SetCharacter(characterFromConfig(m.cfg.Mascot.Style))
}

func (m Model) activeToolID() string {
	if m.tool != nil {
		return m.tool.tool.id
	}
	return ""
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
	if m.pop.IsOpen() {
		cog = m.styles.IconBtn.Foreground(m.styles.P.Accent).BorderForeground(m.styles.P.Accent).Render(cogLabel)
	}
	brand := m.styles.Brand.Render("◤ HANDY TOOLS") + m.styles.Sub.Render(" / htools")
	crumbTitle := "Home"
	if m.current == PageTool && m.tool != nil {
		crumbTitle = m.tool.tool.label
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

	// left column. Cap the expanded stderr panel so a tall job log can't
	// push the rest of the UI past the terminal viewport on short windows.
	// The fixed overhead (header + status + mascot card + state card +
	// queue framing) is roughly 32 rows; whatever's left of m.height is
	// the budget for log entries inside the expanded panel. Floored at 2
	// so the title + omission marker stay visible, and skipped entirely
	// (0) before WindowSizeMsg arrives so first paint renders uncapped.
	logBudget := 0
	if m.height > 0 {
		logBudget = m.height - 32
		if logBudget < 2 {
			logBudget = 2
		}
	}
	leftCol := lipgloss.JoinVertical(lipgloss.Left,
		m.mascot.View(),
		stateBlock(m.styles, m.mascot.State(), m.currentTask, m.progress, left),
		queueView(m.styles, m.queue, left, -1, logBudget),
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
	}
	rightCol = lipgloss.NewStyle().Width(rightWidth).Render(rightCol)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)
	status := m.renderStatusBar()

	// When the settings popover is open it slips between the header and the
	// body — anchored top-left near the cog, indented by 2 cols to feel
	// "floating" without true overlay rendering (which lipgloss can't do
	// cleanly through ANSI styles).
	parts := []string{header, ""}
	if m.pop.IsOpen() {
		popWidth := intMax(28, m.width/3)
		if popWidth > 56 {
			popWidth = 56
		}
		card := m.pop.View(m.styles, m.cfg, popWidth)
		parts = append(parts, lipgloss.NewStyle().PaddingLeft(2).Render(card), "")
	}
	parts = append(parts, body, "", status)
	frame := lipgloss.JoinVertical(lipgloss.Left, parts...)
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
	if m.pop.IsOpen() {
		parts = append(parts,
			chip("↑↓", "move"),
			chip("↵", "cycle"),
			chip("esc", "close"),
		)
	} else {
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
		}
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
