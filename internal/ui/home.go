package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy-tools/internal/ui/mascot"
	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
)

// toolMode is how the tool detail page renders its inputs.
type toolMode int

const (
	modeImage toolMode = iota
	modePackArchive
	modeExtractArchive
	modePDF
	modeDoctor
)

// tool is one entry in the home catalog. It carries everything the home menu
// and the tool detail page need so we don't re-do lookups when switching.
type tool struct {
	id    string
	glyph string
	label string
	desc  string
	mode  toolMode
}

var defaultTools = []tool{
	{id: "convert-image", glyph: "◇", label: "Convert images", desc: "PNG · JPEG · WebP · GIF · BMP · TIFF", mode: modeImage},
	{id: "zip-pack", glyph: "▢", label: "Pack into archive", desc: "zip · tar.gz · tar.bz2 · zstd · 7z", mode: modePackArchive},
	{id: "archive-extract", glyph: "◰", label: "Extract archive", desc: "zip · 7z · rar · tar · gz · bz2 · zst", mode: modeExtractArchive},
	{id: "pdf", glyph: "◫", label: "PDF utilities", desc: "merge · split · pages → image · text", mode: modePDF},
	{id: "doctor", glyph: "◊", label: "Doctor", desc: "check optional system tools", mode: modeDoctor},
}

// speechForState returns the per-state speech bubble line under the mascot
// greeting, mirroring the speech switch from the design's app.jsx. When the
// state is neutral (idle), the line falls back to the per-tool copy from
// speechFor. The `progress` arg is only consulted for the watching state; it
// can be any value otherwise.
func speechForState(state mascot.State, toolID string, toolLabel string, progress int) string {
	switch state {
	case mascot.StateThinking:
		if toolID != "" {
			return "Pulling up the inputs and options…"
		}
		return "Hmm, thinking…"
	case mascot.StateWatching:
		if toolLabel != "" {
			return fmt.Sprintf("Watching %s — %d%% done.", strings.ToLower(toolLabel), progress)
		}
		return "Watching the queue."
	case mascot.StateStressed:
		return "Lots in flight — holding it together."
	case mascot.StateTired:
		return "Ugh, these last ones are dragging — almost done."
	case mascot.StateHappy:
		return "All done. Files are in './out'."
	case mascot.StateWorried:
		return "Something in the queue failed. I'm keeping an eye on it."
	}
	return speechFor(toolID)
}

// speechFor returns the per-tool speech bubble line under the mascot greeting.
func speechFor(id string) string {
	switch id {
	case "convert-image":
		return "Throw any image at me — I'll re-encode it."
	case "zip-pack":
		return "I'll bundle everything into a tidy archive."
	case "archive-extract":
		return "Even multi-part .7z.001 — I'll stitch them."
	case "pdf":
		return "Merge, split, render pages, pull text."
	case "doctor":
		return "Let's see which optional binaries are on PATH."
	}
	return "Pick a tool to get started."
}

type homePage struct {
	styles theme.Styles
	cursor int
	width  int
}

func newHomePage(s theme.Styles) Page { return &homePage{styles: s} }

func (h *homePage) Init() tea.Cmd { return nil }
func (h *homePage) ID() PageID    { return PageHome }
func (h *homePage) Title() string { return "Home" }

func (h *homePage) SetWidth(w int) { h.width = w }
func (h *homePage) Cursor() int    { return h.cursor }
func (h *homePage) SetCursor(c int) {
	if c < 0 {
		c = 0
	}
	if c >= len(defaultTools) {
		c = len(defaultTools) - 1
	}
	h.cursor = c
}

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
		case "1", "2", "3", "4", "5":
			i := int(k.String()[0] - '1')
			if i >= 0 && i < len(defaultTools) {
				h.cursor = i
			}
		case "enter":
			t := defaultTools[h.cursor]
			return h, func() tea.Msg { return OpenTool{ID: t.id} }
		}
	}
	return h, nil
}

func (h *homePage) View() string {
	s := h.styles
	width := h.width
	if width <= 0 {
		width = 80
	}

	tagline := lipgloss.NewStyle().Foreground(s.P.TextDim).Render(
		"Welcome to ") + s.Accent.Bold(true).Render("Handy Tools") +
		lipgloss.NewStyle().Foreground(s.P.TextDim).Render(
			" — a friendly toolbox for everyday file work.")

	help := s.Dim.Render(
		"Pick a tool to set up an input, output and options. ") +
		s.KbdChip.Render("↑↓") + " " + s.Dim.Render("/") + " " +
		s.KbdChip.Render("ENTER")

	section := lipgloss.JoinHorizontal(lipgloss.Top,
		s.Section.Render("AVAILABLE TOOLS"),
		"  ",
		lipgloss.NewStyle().Foreground(s.P.Border).Render(
			strings.Repeat("─", intMax(4, width-32))),
		"  ",
		s.Dim.Render(
			itoa(h.cursor+1)+" / "+itoa(len(defaultTools))),
	)

	rows := []string{tagline, help, "", section, ""}
	for i, t := range defaultTools {
		rows = append(rows, h.renderRow(t, i == h.cursor, width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (h *homePage) renderRow(t tool, active bool, width int) string {
	s := h.styles
	cursor := "  "
	glyph := lipgloss.NewStyle().Foreground(s.P.TextDim).Bold(true).Render(t.glyph)
	label := lipgloss.NewStyle().Foreground(s.P.Text).Bold(true).Render(t.label)
	desc := s.Dim.Render(" — " + t.desc)
	arrow := lipgloss.NewStyle().Foreground(s.P.TextDim).Render("  ")

	if active {
		cursor = s.Accent.Bold(true).Render("▸ ")
		glyph = s.Accent.Bold(true).Render(t.glyph)
		arrow = s.Accent.Render(" ↵")
	}

	inner := lipgloss.JoinHorizontal(lipgloss.Top, cursor, glyph, "  ", label, desc)
	rowWidth := intMax(20, width-4)
	pad := rowWidth - lipgloss.Width(inner) - lipgloss.Width(arrow)
	if pad < 1 {
		pad = 1
	}
	line := inner + strings.Repeat(" ", pad) + arrow

	style := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(s.P.Background) // invisible border so width matches active row
	if active {
		style = style.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(s.P.Accent).
			Background(s.P.Surface)
	}
	return style.Render(line)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
