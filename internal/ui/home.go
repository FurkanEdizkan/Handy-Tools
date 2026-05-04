package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy/internal/ui/theme"
)

type tool struct {
	id    string
	label string
	desc  string
	page  PageID
}

var defaultTools = []tool{
	{id: "files", label: "Files", desc: "Browse and act on files", page: PageFiles},
	{id: "image", label: "Convert images", desc: "PNG, JPEG, WebP, ...", page: PageFiles},
	{id: "archive", label: "Extract archives", desc: "zip, 7z, rar, tar", page: PageFiles},
	{id: "pdf", label: "PDF tools", desc: "Convert, merge, split", page: PageFiles},
	{id: "settings", label: "Settings", desc: "Theme, defaults, mascot", page: PageSettings},
}

type homePage struct {
	styles theme.Styles
	cursor int
}

func newHomePage(s theme.Styles) Page { return &homePage{styles: s} }

func (h *homePage) Init() tea.Cmd                  { return nil }
func (h *homePage) ID() PageID                     { return PageHome }
func (h *homePage) Title() string                  { return "Home" }
func (h *homePage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			if h.cursor > 0 {
				h.cursor--
			}
		case "down", "j":
			if h.cursor < len(defaultTools)-1 {
				h.cursor++
			}
		case "enter":
			return h, func() tea.Msg { return Navigate{To: defaultTools[h.cursor].page} }
		}
	}
	return h, nil
}

func (h *homePage) View() string {
	rows := make([]string, 0, len(defaultTools))
	for i, t := range defaultTools {
		body := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(t.label),
			lipgloss.NewStyle().Foreground(h.styles.P.TextDim).Render(t.desc),
		)
		card := h.styles.Card
		if i == h.cursor {
			card = h.styles.CardActive
		}
		rows = append(rows, card.Render(body))
	}
	hint := h.styles.Status.Render("up/down  -  enter to open")
	return lipgloss.JoinVertical(lipgloss.Left, append(rows, hint)...)
}
