// Package mascot renders the red-panda companion. It exposes a Bubble Tea
// component-shaped struct (Update + View) so any page can embed one and
// flip its state via Set(state).
//
// The character is Wrenly: a small red panda rendered in pure ASCII as a
// stylized face only — ears, eyes, nose, mouth, chin. Per-state frames give
// idle/thinking/working/success/error a distinct expression so the animation
// hook is already in place. Pure ASCII keeps every column locked in any
// monospace terminal and stays trivial to lift onto other surfaces.
package mascot

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
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
	speech   string
	styles   theme.Styles
	width    int
}

// New returns a mascot in the idle state.
func New(s theme.Styles) Model {
	return Model{state: StateIdle, styles: s, greeting: "Hi! I'm Wrenly."}
}

// Set switches the mascot to the given state.
func (m *Model) Set(s State) { m.state = s; m.frame = 0 }

// State reports the current animation state.
func (m Model) State() State { return m.state }

// Say replaces the greeting text shown under the face.
func (m *Model) Say(text string) { m.greeting = text }

// Whisper replaces the secondary speech line (lower-contrast).
func (m *Model) Whisper(text string) { m.speech = text }

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

// face renders the four-line Wrenly face for the given eyes / mouth.
//
//	 /\___/\
//	( L . R )
//	 \  M  /
//	  `---`
func face(leftEye, rightEye, mouth string, decor string) string {
	row1 := "   /\\___/\\"
	if decor != "" {
		row1 += "   " + decor
	}
	return strings.Join([]string{
		row1,
		"  ( " + leftEye + " . " + rightEye + " )",
		"   \\  " + mouth + "  /",
		"    `---`",
	}, "\n")
}

// art returns the multi-line ASCII for the current state and animation frame.
// Animation cycles a small set of frames per state.
func (m Model) art() string {
	switch m.state {
	case StateThinking:
		// alternate winks with a hovering '?'
		switch m.frame % 2 {
		case 0:
			return face("o", "-", "~", "?")
		default:
			return face("-", "o", "~", "?")
		}
	case StateWorking:
		// alternate "focused" and "happy-effort" frames
		if m.frame%2 == 0 {
			return face("o", "o", "o", "")
		}
		return face("^", "^", "o", "")
	case StateSuccess:
		// sparkles dance left/right around the head
		switch m.frame % 3 {
		case 0:
			return face("^", "^", "w", "*")
		case 1:
			return face("^", "^", "w", "")
		default:
			return face("^", "^", "w", "*")
		}
	case StateError:
		return face("x", "x", "_", "")
	}
	// idle: subtle blink / look-around cycle
	switch m.frame % 4 {
	case 0:
		return face("o", "o", "v", "")
	case 1:
		return face("-", "-", "v", "")
	case 2:
		return face("o", "O", "v", "")
	default:
		return face("O", "o", "v", "")
	}
}

// String returns the lowercase name of a State ("idle", "thinking", …).
func (s State) String() string {
	switch s {
	case StateThinking:
		return "thinking"
	case StateWorking:
		return "working"
	case StateSuccess:
		return "done"
	case StateError:
		return "error"
	}
	return "idle"
}

// StatusLabel returns a short uppercase tag for the current state — used by
// the surrounding TUI to render a "wrenly · WORKING" badge.
func (m Model) StatusLabel() string {
	switch m.state {
	case StateThinking:
		return "THINKING"
	case StateWorking:
		return "WORKING"
	case StateSuccess:
		return "DONE"
	case StateError:
		return "ERROR"
	}
	return "IDLE"
}

// View renders the mascot inside a card with the badge + face + greeting.
func (m Model) View() string {
	furStyle := m.styles.MascotFur.Bold(true)
	badge := lipgloss.NewStyle().Foreground(m.styles.P.TextDim).Render("wrenly · ") +
		lipgloss.NewStyle().Foreground(m.styles.P.Accent).Bold(true).Render(m.StatusLabel())
	body := furStyle.Render(m.art())
	greet := lipgloss.NewStyle().Foreground(m.styles.P.Text).Bold(true).Render(m.greeting)

	parts := []string{badge, "", body, "", greet}
	if m.speech != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(m.styles.P.TextDim).Render(m.speech))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if m.width > 0 {
		return m.styles.Card.Width(m.width).Render(content)
	}
	return m.styles.Card.Render(content)
}
