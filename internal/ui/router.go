// Package ui hosts the Bubble Tea models. The top-level Router stacks pages
// (Home, FileBrowser, ToolView, Settings); each page is itself a tea.Model.
//
// The router keeps the mascot global: pages mutate it via tea.Cmd messages
// rather than each holding their own.
package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy/internal/config"
	"github.com/furkandedizkan/handy/internal/ui/mascot"
	"github.com/furkandedizkan/handy/internal/ui/theme"
)

// PageID identifies a top-level page.
type PageID int

const (
	PageHome PageID = iota
	PageFiles
	PageSettings
)

// Page is the contract every TUI screen implements.
type Page interface {
	Init() tea.Cmd
	Update(tea.Msg) (Page, tea.Cmd)
	View() string
	ID() PageID
	Title() string
}

// Navigate is sent by a page to swap the active page.
type Navigate struct{ To PageID }

// MascotMsg lets pages drive the global mascot.
type MascotMsg struct {
	State    mascot.State
	Greeting string
}

// Model is the top-level Bubble Tea program.
type Model struct {
	cfg      config.Config
	styles   theme.Styles
	mascot   mascot.Model
	pages    map[PageID]Page
	current  PageID
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
	}
	m.pages[PageHome] = newHomePage(styles)
	m.pages[PageFiles] = newFilesPage(styles)
	m.pages[PageSettings] = newSettingsPage(styles, &m.cfg)
	return m
}

// Init begins the mascot animation loop.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{mascot.Tick()}
	for _, p := range m.pages {
		if c := p.Init(); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

// Update routes messages: window size and key shortcuts (q to quit, tab to
// cycle pages) are handled here; the rest goes to the active page.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.mascot.SetWidth(min(36, m.width/3))
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			m.current = nextPage(m.current)
			return m, nil
		case "shift+tab":
			m.current = prevPage(m.current)
			return m, nil
		}
	case Navigate:
		m.current = msg.To
		return m, nil
	case MascotMsg:
		m.mascot.Set(msg.State)
		if msg.Greeting != "" {
			m.mascot.Say(msg.Greeting)
		}
		return m, nil
	}

	// always forward to mascot for animation ticks
	mm, mc := m.mascot.Update(msg)
	m.mascot = mm

	page := m.pages[m.current]
	updated, pc := page.Update(msg)
	m.pages[m.current] = updated

	return m, tea.Batch(mc, pc)
}

// View composes the active page with the mascot pane on the right.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	page := m.pages[m.current]
	header := m.styles.Title.Render("Handy  -  " + page.Title())
	help := m.styles.Status.Render("tab: switch page  -  q: quit")

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(maxInt(20, m.width-40)).Render(page.View()),
		m.mascot.View(),
	)
	return m.styles.App.Render(lipgloss.JoinVertical(lipgloss.Left, header, body, help))
}

func nextPage(p PageID) PageID {
	switch p {
	case PageHome:
		return PageFiles
	case PageFiles:
		return PageSettings
	default:
		return PageHome
	}
}

func prevPage(p PageID) PageID {
	switch p {
	case PageHome:
		return PageSettings
	case PageFiles:
		return PageHome
	default:
		return PageFiles
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
