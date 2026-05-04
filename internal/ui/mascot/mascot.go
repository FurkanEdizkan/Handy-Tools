// Package mascot renders the snow-fox companion. It exposes a Bubble Tea
// component-shaped struct (Update + View) so any page can embed one and
// flip its state via Set(state).
package mascot

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy/internal/ui/theme"
)

// State enumerates the mascot's expression / animation.
type State int

const (
	StateIdle State = iota
	StateThinking
	StateWorking
	StateSuccess
	StateError
)

// Model is the mascot component.
type Model struct {
	state    State
	frame    int
	greeting string
	styles   theme.Styles
	width    int
}

// New returns a mascot in the idle state.
func New(s theme.Styles) Model {
	return Model{state: StateIdle, styles: s, greeting: "Hi! I'm Yuki."}
}

// Set switches the mascot to the given state.
func (m *Model) Set(s State) { m.state = s; m.frame = 0 }

// Say replaces the greeting text.
func (m *Model) Say(text string) { m.greeting = text }

// SetStyles updates the palette (e.g. after the user changes themes).
func (m *Model) SetStyles(s theme.Styles) { m.styles = s }

// SetWidth tells the mascot how much horizontal space it has.
func (m *Model) SetWidth(w int) { m.width = w }

type tickMsg time.Time

// Tick returns a command that advances the mascot's animation frame.
func Tick() tea.Cmd {
	return tea.Tick(time.Millisecond*350, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update advances animation and consumes tickMsg.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if _, ok := msg.(tickMsg); ok {
		m.frame++
		return m, Tick()
	}
	return m, nil
}

// art returns the multi-line ASCII for the current state and animation frame.
func (m Model) art() string {
	switch m.state {
	case StateThinking:
		return foxBaseWithEyes("?")
	case StateWorking:
		dots := strings.Repeat(".", (m.frame%3)+1)
		return foxBaseWithEyes("o") + "\n  working" + dots
	case StateSuccess:
		return foxBaseWithEyes("^")
	case StateError:
		return foxBaseWithEyes("x")
	}
	return foxBaseWithEyes("o")
}

// foxBaseWithEyes returns a small snow-fox face. eye is the character drawn
// in the eye slots so we can switch expression without redrawing the whole
// figure.
func foxBaseWithEyes(eye string) string {
	return "" +
		"   /\\___/\\\n" +
		"  ( " + eye + "   " + eye + " )\n" +
		"   ) ^ (   \n" +
		"  (  v  )\n" +
		"   \\___/\n"
}

// View renders the mascot inside a card.
func (m Model) View() string {
	body := m.styles.MascotFur.Render(m.art())
	caption := lipgloss.NewStyle().Foreground(m.styles.P.TextDim).Render(m.greeting)
	content := lipgloss.JoinVertical(lipgloss.Left, body, caption)
	if m.width > 0 {
		return m.styles.Card.Width(m.width).Render(content)
	}
	return m.styles.Card.Render(content)
}
