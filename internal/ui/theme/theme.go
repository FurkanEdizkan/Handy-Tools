// Package theme defines named Lip Gloss styles. The TUI gets its colors from
// here so changing the theme is one place.
package theme

import "github.com/charmbracelet/lipgloss"

// Palette describes a single named theme.
type Palette struct {
	Name        string
	Background  lipgloss.Color
	Surface     lipgloss.Color
	Border      lipgloss.Color
	Accent      lipgloss.Color
	AccentSoft  lipgloss.Color
	Text        lipgloss.Color
	TextDim     lipgloss.Color
	Success     lipgloss.Color
	Warning     lipgloss.Color
	Error       lipgloss.Color
	MascotFur   lipgloss.Color
	MascotEye   lipgloss.Color
}

// Forge is the default orange-on-black palette: black background, deep amber
// accents, warm rust mascot fur. It matches the installer's ANSI banner and
// the brand colors used by Handy Tools across surfaces (TUI, future web,
// Chrome extension).
var Forge = Palette{
	Name:       "forge",
	Background: lipgloss.Color("#0a0807"),
	Surface:    lipgloss.Color("#141110"),
	Border:     lipgloss.Color("#3a2a1f"),
	Accent:     lipgloss.Color("#ff8c1a"),
	AccentSoft: lipgloss.Color("#a35a14"),
	Text:       lipgloss.Color("#f5ece2"),
	TextDim:    lipgloss.Color("#9a877a"),
	Success:    lipgloss.Color("#9bdc8a"),
	Warning:    lipgloss.Color("#f0c674"),
	Error:      lipgloss.Color("#e57373"),
	MascotFur:  lipgloss.Color("#ff8c1a"),
	MascotEye: lipgloss.Color("#1a0f08"),
}

// Snow is a cool blue-black palette (kept for users who prefer it).
var Snow = Palette{
	Name:       "snow",
	Background: lipgloss.Color("#0b0f14"),
	Surface:    lipgloss.Color("#11161d"),
	Border:     lipgloss.Color("#1f2630"),
	Accent:     lipgloss.Color("#7ad7f0"),
	AccentSoft: lipgloss.Color("#3a6f7e"),
	Text:       lipgloss.Color("#e6ecf2"),
	TextDim:    lipgloss.Color("#8a94a3"),
	Success:    lipgloss.Color("#9bdc8a"),
	Warning:    lipgloss.Color("#f0c674"),
	Error:      lipgloss.Color("#e57373"),
	MascotFur:  lipgloss.Color("#f1f5fa"),
	MascotEye:  lipgloss.Color("#7ad7f0"),
}

// Ember is a warm alternative with softer orange tones.
var Ember = Palette{
	Name:       "ember",
	Background: lipgloss.Color("#140d0a"),
	Surface:    lipgloss.Color("#1d1410"),
	Border:     lipgloss.Color("#3a2a22"),
	Accent:     lipgloss.Color("#f0a05a"),
	AccentSoft: lipgloss.Color("#7e4a30"),
	Text:       lipgloss.Color("#f4ece4"),
	TextDim:    lipgloss.Color("#a39084"),
	Success:    lipgloss.Color("#9bdc8a"),
	Warning:    lipgloss.Color("#f0c674"),
	Error:      lipgloss.Color("#e57373"),
	MascotFur:  lipgloss.Color("#f4ece4"),
	MascotEye:  lipgloss.Color("#f0a05a"),
}

// Resolve returns the palette by name; falls back to Forge.
func Resolve(name string) Palette {
	switch name {
	case "snow":
		return Snow
	case "ember":
		return Ember
	case "forge":
		return Forge
	default:
		return Forge
	}
}

// Styles bundles the most-used Lip Gloss styles for a palette.
type Styles struct {
	P           Palette
	App         lipgloss.Style
	Title       lipgloss.Style
	Card        lipgloss.Style
	CardActive  lipgloss.Style
	HelpKey     lipgloss.Style
	HelpDesc    lipgloss.Style
	Status      lipgloss.Style
	Error       lipgloss.Style
	MascotFur   lipgloss.Style
	MascotEye   lipgloss.Style
}

// Build returns a Styles populated from p.
func Build(p Palette) Styles {
	base := lipgloss.NewStyle().Background(p.Background).Foreground(p.Text)
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Border).
		Background(p.Surface).
		Foreground(p.Text).
		Padding(0, 1)
	return Styles{
		P:          p,
		App:        base,
		Title:      base.Bold(true).Foreground(p.Accent).Padding(0, 1),
		Card:       card,
		CardActive: card.Copy().BorderForeground(p.Accent).Foreground(p.Text),
		HelpKey:    lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		HelpDesc:   lipgloss.NewStyle().Foreground(p.TextDim),
		Status:     lipgloss.NewStyle().Foreground(p.TextDim),
		Error:      lipgloss.NewStyle().Foreground(p.Error).Bold(true),
		MascotFur:  lipgloss.NewStyle().Foreground(p.MascotFur),
		MascotEye:  lipgloss.NewStyle().Foreground(p.MascotEye),
	}
}
