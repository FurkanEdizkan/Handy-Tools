// Package mascot renders a small state-indicator companion next to the queue
// panel. It exposes a Bubble Tea component-shaped struct (Update + View) so
// any page can embed one and flip its state via Set(state).
//
// The art is a colored dot-grid sprite — Wrenly the orange panda by default,
// Hopper the lilac rabbit if the user opts in via Settings. The sprite legend
// (O / b / w / E / N / M) and the per-state behavior (idle / thinking /
// watching / stressed / tired / happy / worried) come from the Claude Design
// handoff bundle. Each state tints the fur, swaps the eye glyph, and adds a
// small overlay above the head (thought dots, stress sparks, droop marks).
//
// Pure ASCII / Unicode block glyphs keep every column locked in a monospace
// terminal and stay trivial to lift onto other surfaces (web, Chrome
// extension — see docs/ROADMAP.md).
package mascot

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
)

// State enumerates the mascot's expression / animation. The numeric values
// are stable; older callers that referenced StateWorking / StateSuccess /
// StateError still compile thanks to the aliases below.
type State int

const (
	StateIdle State = iota
	StateThinking
	StateWatching
	StateStressed
	StateTired
	StateHappy
	StateWorried
)

// Back-compat aliases. Earlier code paths used the working/success/error
// vocabulary; the design renamed those to watching/happy/worried. Keep the
// old identifiers pointing at the closest new state so we don't churn every
// call site at once.
const (
	StateWorking = StateWatching
	StateSuccess = StateHappy
	StateError   = StateWorried
)

// Character identifies which sprite to render.
const (
	CharacterWrenly = "wrenly"
	CharacterHopper = "hopper"
)

// Sprite legend (from the design's mascot.jsx):
//
//	.  empty (also ' ')
//	O  fur (main accent)
//	b  dark stripe / outline
//	w  cream / inner ear / muzzle
//	E  eye
//	N  nose
//	M  mouth
//
// Wrenly — 15×14 panda with cheek mask + tear stripes + 3-dot nose.
const wrenlySprite = `.O...........O.
OOO.........OOO
OwwO.......OwwO
OOOOOOOOOOOOOOO
OOOOOOOOOOOOOOO
OwwOOOOOOOOOwwO
OwwbEbwOwbEbwwO
OwbwwwwOwwwwbwO
OwwwwwwwwwwwwwO
OwwwwwNNNwwwwwO
OwwwwwbMbwwwwwO
.OwwwwwwwwwwwO.
..OOwwwwwwwOO..
.....OOOOO.....`

// Hopper — 15×15 rabbit with tall pink-inner ears, bead eyes, pink nose, tiny
// mouth. No tear stripes (no 'b' in the eye row).
const hopperSprite = `.O...........O.
OwO.........OwO
OwO.........OwO
OwO.........OwO
OwO.........OwO
OOO.........OOO
.OOOOOOOOOOOOO.
OOOOOOOOOOOOOOO
OOOEOOOOOOOEOOO
OwwwwwwwwwwwwwO
OwwwwwbNbwwwwwO
OwwwwwwMwwwwwwO
.OwwwwwwwwwwwO.
..OOwwwwwwwOO..
....OOOOOOO....`

// Glyphs picked per sprite token. Two characters wide (a block + space) so
// the sprite reads square-ish in a terminal where cells are taller than wide.
const (
	glyphFur    = "● "
	glyphStripe = "● "
	glyphCream  = "○ "
	glyphNose   = "● "
	glyphMouth  = "▪ "
	glyphEmpty  = "  "
)

// hopperFur is the lilac fur tint Hopper keeps across every theme — the
// design's `.character-hopper { --mascot-fur: #cdb9ed }`.
const (
	hopperFur  = lipgloss.Color("#cdb9ed")
	hopperPink = lipgloss.Color("#ff8fb8")
	hopperEar  = lipgloss.Color("#ffe6ef")
)

// Model is the mascot component.
type Model struct {
	state     State
	character string
	frame     int
	greeting  string
	speech    string
	styles    theme.Styles
	width     int
	maxHeight int
}

// New returns a mascot in the idle state for the default character (Wrenly).
func New(s theme.Styles) Model {
	return Model{
		state:     StateIdle,
		character: CharacterWrenly,
		styles:    s,
		greeting:  "Ready when you are.",
	}
}

// Set switches the mascot to the given state. The frame counter resets so
// per-state animations start from their first frame.
func (m *Model) Set(s State) { m.state = s; m.frame = 0 }

// State reports the current animation state.
func (m Model) State() State { return m.state }

// Say replaces the greeting text shown under the frame.
func (m *Model) Say(text string) { m.greeting = text }

// Whisper replaces the secondary speech line (lower-contrast).
func (m *Model) Whisper(text string) { m.speech = text }

// SetStyles updates the palette (e.g. after the user changes themes).
func (m *Model) SetStyles(s theme.Styles) { m.styles = s }

// SetWidth tells the mascot how much horizontal space it has.
func (m *Model) SetWidth(w int) { m.width = w }

// SetMaxHeight gives the mascot a hint about how many rows it may use
// in the left column. 0 leaves the mascot uncapped (legacy behavior).
// When the budget is smaller than the full sprite + chrome, View()
// drops to a compact rendering: badge + greeting + one-line speech.
func (m *Model) SetMaxHeight(h int) { m.maxHeight = h }

// SetCharacter picks Wrenly or Hopper. Unknown values fall back to Wrenly.
func (m *Model) SetCharacter(c string) {
	switch c {
	case CharacterHopper:
		m.character = CharacterHopper
	default:
		m.character = CharacterWrenly
	}
}

// Character reports the current character key.
func (m Model) Character() string { return m.character }

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

// String returns the lowercase name of a State.
func (s State) String() string {
	switch s {
	case StateThinking:
		return "thinking"
	case StateWatching:
		return "watching"
	case StateStressed:
		return "stressed"
	case StateTired:
		return "tired"
	case StateHappy:
		return "happy"
	case StateWorried:
		return "worried"
	}
	return "idle"
}

// StatusLabel returns a short uppercase tag for the current state — used by
// the surrounding TUI to render a "STATE · WATCHING" badge. Matches the
// STATUS map from the design's mascot.jsx (note: tired is shown as
// FRUSTRATED, the original artist's choice).
func (m Model) StatusLabel() string {
	switch m.state {
	case StateThinking:
		return "THINKING"
	case StateWatching:
		return "WATCHING"
	case StateStressed:
		return "STRESSED"
	case StateTired:
		return "FRUSTRATED"
	case StateHappy:
		return "HAPPY"
	case StateWorried:
		return "WORRIED"
	}
	return "IDLE"
}

// Sprite returns the multi-line dot-grid art for the named character using
// the design's plain glyphs (● ○ ▪) with no ANSI color codes — surfaces that
// can't run lipgloss (README hero, install.sh banner, brand assets generated
// by cmd/snapshot) use this so they don't drift from the in-TUI mascot.
// Unknown character values fall back to Wrenly.
func Sprite(character string) string {
	sprite := wrenlySprite
	if character == CharacterHopper {
		sprite = hopperSprite
	}
	var b strings.Builder
	for i, row := range strings.Split(sprite, "\n") {
		if i > 0 {
			b.WriteString("\n")
		}
		for _, r := range row {
			switch r {
			case 'O', 'b':
				b.WriteString(glyphFur)
			case 'w':
				b.WriteString(glyphCream)
			case 'E':
				b.WriteString("• ")
			case 'N':
				b.WriteString(glyphNose)
			case 'M':
				b.WriteString(glyphMouth)
			default:
				b.WriteString(glyphEmpty)
			}
		}
	}
	return b.String()
}

// art renders the multi-line sprite for the current state, character, and
// animation frame, with per-token color applied via lipgloss.
func (m Model) art() string {
	sprite := wrenlySprite
	if m.character == CharacterHopper {
		sprite = hopperSprite
	}

	fur := furColor(m.state, m.styles, m.character)
	stripe := m.styles.P.Border
	cream := m.styles.P.Text
	nose := m.styles.P.Accent
	pupil := m.styles.P.Background
	mouth := m.styles.P.TextDim

	if m.character == CharacterHopper {
		nose = hopperPink
		cream = hopperEar
	}

	furStyle := lipgloss.NewStyle().Foreground(fur)
	stripeStyle := lipgloss.NewStyle().Foreground(stripe)
	creamStyle := lipgloss.NewStyle().Foreground(cream)
	noseStyle := lipgloss.NewStyle().Foreground(nose)
	pupilStyle := lipgloss.NewStyle().Foreground(pupil).Background(fur)
	mouthStyle := lipgloss.NewStyle().Foreground(mouth)

	eye := eyeGlyph(m.state, m.frame)

	lines := strings.Split(sprite, "\n")
	out := make([]string, 0, len(lines)+1)

	if overlay := overlayLine(m.state, m.frame, m.styles); overlay != "" {
		out = append(out, overlay)
	} else {
		out = append(out, "")
	}

	for _, row := range lines {
		var b strings.Builder
		for _, r := range row {
			switch r {
			case 'O':
				b.WriteString(furStyle.Render(glyphFur))
			case 'b':
				b.WriteString(stripeStyle.Render(glyphStripe))
			case 'w':
				b.WriteString(creamStyle.Render(glyphCream))
			case 'E':
				b.WriteString(pupilStyle.Render(eye))
			case 'N':
				b.WriteString(noseStyle.Render(glyphNose))
			case 'M':
				b.WriteString(mouthStyle.Render(glyphMouth))
			default:
				b.WriteString(glyphEmpty)
			}
		}
		out = append(out, b.String())
	}
	return strings.Join(out, "\n")
}

// furColor returns the per-state fur tint. Hopper keeps its lilac base tone
// in calm states and shares the tinted hues used for stress / happy / tired
// / worried so the mood shift still reads.
func furColor(state State, styles theme.Styles, character string) lipgloss.Color {
	switch state {
	case StateStressed:
		return styles.P.Accent
	case StateTired:
		return styles.P.TextDim
	case StateHappy:
		return styles.P.Success
	case StateWorried:
		return styles.P.Error
	}
	if character == CharacterHopper {
		return hopperFur
	}
	return styles.P.MascotFur
}

// eyeGlyph picks the per-state eye glyph (block + space, so it occupies a
// full sprite cell). Idle blinks subtly every few frames.
func eyeGlyph(state State, frame int) string {
	switch state {
	case StateHappy:
		return "‿ "
	case StateWorried:
		return "⌒ "
	case StateStressed:
		// wide, quivering eye — alternate slightly per-frame for shake
		if frame%2 == 0 {
			return "◉ "
		}
		return "● "
	case StateTired:
		return "- "
	case StateThinking:
		// glance up-right: pupil sits slightly off-center via a narrower glyph
		return "˙ "
	case StateIdle:
		// random-ish blink: closed on every 7th-ish frame
		if frame%7 == 6 {
			return "- "
		}
		return "• "
	}
	return "• "
}

// overlayLine produces the one-row decoration that floats above the sprite:
// thought dots when thinking, sparks when happy/stressed, sweat when
// worried, huff marks when tired. Empty string falls through to a blank row.
func overlayLine(state State, frame int, styles theme.Styles) string {
	switch state {
	case StateThinking:
		dim := lipgloss.NewStyle().Foreground(styles.P.TextDim).Render
		accent := lipgloss.NewStyle().Foreground(styles.P.Accent).Render
		// little float-up animation: cycle the bullet sizes
		switch frame % 3 {
		case 0:
			return strings.Repeat(" ", 22) + dim("·") + " " + accent("°") + " " + accent("●")
		case 1:
			return strings.Repeat(" ", 22) + accent("·") + " " + accent("●") + " " + dim("∘")
		default:
			return strings.Repeat(" ", 22) + accent("●") + " " + dim("∘") + " " + dim(" ")
		}
	case StateHappy:
		acc := lipgloss.NewStyle().Foreground(styles.P.Success).Render
		// sparkle pattern that shifts horizontally
		switch frame % 3 {
		case 0:
			return "   " + acc("✦") + "       " + acc("✧") + "        " + acc("✦")
		case 1:
			return "      " + acc("✧") + "       " + acc("✦") + "     " + acc("✧")
		default:
			return "  " + acc("✦") + "         " + acc("✧") + "      " + acc("✦")
		}
	case StateStressed:
		acc := lipgloss.NewStyle().Foreground(styles.P.Accent).Bold(true).Render
		// sharp ticks around the head — flicker
		if frame%2 == 0 {
			return " " + acc("╱") + "                          " + acc("╲")
		}
		return acc("╱") + "                            " + acc("╲")
	case StateWorried:
		err := lipgloss.NewStyle().Foreground(styles.P.Error).Render
		// sweat drop wobbles at the right temple
		if frame%2 == 0 {
			return strings.Repeat(" ", 26) + err("💧")
		}
		return strings.Repeat(" ", 27) + err("💧")
	case StateTired:
		dim := lipgloss.NewStyle().Foreground(styles.P.TextDim).Render
		// huff marks at the temple
		return strings.Repeat(" ", 24) + dim("~ ~")
	}
	return ""
}

// View renders the mascot inside a card with the badge + sprite + greeting.
// When SetMaxHeight has been called with a budget too small for the full
// 14-row sprite plus chrome, we drop to a compact form (badge, greeting,
// speech) so the surrounding two-pane layout still fits the terminal
// viewport on short windows.
func (m Model) View() string {
	badge := lipgloss.NewStyle().Foreground(m.styles.P.TextDim).Render(m.character+" · ") +
		lipgloss.NewStyle().Foreground(m.styles.P.Accent).Bold(true).Render(m.StatusLabel())
	greet := lipgloss.NewStyle().Foreground(m.styles.P.Text).Bold(true).Render(m.greeting)

	// Full layout needs ~22 rows once the card border (2) and inter-card
	// spacers are counted; if the budget is below that, render compactly.
	compact := m.maxHeight > 0 && m.maxHeight < 22

	var parts []string
	if compact {
		parts = []string{badge, "", greet}
		if m.speech != "" {
			parts = append(parts, lipgloss.NewStyle().Foreground(m.styles.P.TextDim).Render(m.speech))
		}
	} else {
		body := m.art()
		parts = []string{badge, "", body, "", greet}
		if m.speech != "" {
			parts = append(parts, lipgloss.NewStyle().Foreground(m.styles.P.TextDim).Render(m.speech))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if m.width > 0 {
		return m.styles.Card.Width(m.width).Render(content)
	}
	return m.styles.Card.Render(content)
}
