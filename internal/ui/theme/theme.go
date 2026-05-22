// Package theme defines named Lip Gloss styles. The TUI gets its colors from
// here so changing the theme is one place.
package theme

import "github.com/charmbracelet/lipgloss"

// Palette describes a single named theme.
type Palette struct {
	Name       string
	Background lipgloss.Color
	Surface    lipgloss.Color
	Border     lipgloss.Color
	Accent     lipgloss.Color
	AccentSoft lipgloss.Color
	Text       lipgloss.Color
	TextDim    lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	MascotFur  lipgloss.Color
	MascotEye  lipgloss.Color
}

// Forge is the default orange-on-black palette: black background, deep amber
// accents, warm rust mascot fur. It matches the installer's ANSI banner and
// the brand colors used by Handy Tools across surfaces (TUI, web, desktop).
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
	MascotEye:  lipgloss.Color("#1a0f08"),
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
	P          Palette
	App        lipgloss.Style
	Title      lipgloss.Style
	Card       lipgloss.Style
	CardActive lipgloss.Style
	HelpKey    lipgloss.Style
	HelpDesc   lipgloss.Style
	Status     lipgloss.Style
	Error      lipgloss.Style
	MascotFur  lipgloss.Style
	MascotEye  lipgloss.Style

	// Extended pieces used by the design-driven layout. These all derive
	// from the palette but are pre-built so pages don't re-construct them.
	Brand     lipgloss.Style // accent + bold + letterspaced wordmark
	Sub       lipgloss.Style // dim secondary label
	Section   lipgloss.Style // section title strip
	IconBtn   lipgloss.Style // header button (settings cog etc.)
	Crumb     lipgloss.Style // breadcrumb text
	KbdChip   lipgloss.Style // small inline shortcut hint
	Toast     lipgloss.Style // floating toast
	BadgeOK   lipgloss.Style // green pill
	BadgeWarn lipgloss.Style // amber pill
	BadgeErr  lipgloss.Style // red pill
	BadgeRun  lipgloss.Style // accent pill
	BadgeMute lipgloss.Style // dim pill
	BarTrack  lipgloss.Style // progress-bar background
	BarFill   lipgloss.Style // progress-bar filled portion
	Accent    lipgloss.Style // accent foreground only
	Dim       lipgloss.Style // dim foreground only
	OK        lipgloss.Style // success foreground only
	Warn      lipgloss.Style // warning foreground only
	Err       lipgloss.Style // error foreground only
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
	pill := lipgloss.NewStyle().Padding(0, 1).Bold(true)
	return Styles{
		P:          p,
		App:        base,
		Title:      base.Bold(true).Foreground(p.Accent).Padding(0, 1),
		Card:       card,
		CardActive: card.BorderForeground(p.Accent).Foreground(p.Text),
		HelpKey:    lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		HelpDesc:   lipgloss.NewStyle().Foreground(p.TextDim),
		Status:     lipgloss.NewStyle().Foreground(p.TextDim),
		Error:      lipgloss.NewStyle().Foreground(p.Error).Bold(true),
		MascotFur:  lipgloss.NewStyle().Foreground(p.MascotFur),
		MascotEye:  lipgloss.NewStyle().Foreground(p.MascotEye),

		Brand:     lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		Sub:       lipgloss.NewStyle().Foreground(p.TextDim),
		Section:   lipgloss.NewStyle().Foreground(p.TextDim).Bold(true),
		IconBtn:   lipgloss.NewStyle().Foreground(p.TextDim).Background(p.Surface).Padding(0, 1).Border(lipgloss.NormalBorder(), false).BorderForeground(p.Border),
		Crumb:     lipgloss.NewStyle().Foreground(p.TextDim),
		KbdChip:   lipgloss.NewStyle().Foreground(p.Accent).Bold(true).Padding(0, 1),
		Toast:     lipgloss.NewStyle().Foreground(p.Text).Background(p.Surface).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(p.Accent),
		BadgeOK:   pill.Foreground(p.Success),
		BadgeWarn: pill.Foreground(p.Warning),
		BadgeErr:  pill.Foreground(p.Error),
		BadgeRun:  pill.Foreground(p.Accent),
		BadgeMute: pill.Foreground(p.TextDim),
		BarTrack:  lipgloss.NewStyle().Foreground(p.Border),
		BarFill:   lipgloss.NewStyle().Foreground(p.Accent),
		Accent:    lipgloss.NewStyle().Foreground(p.Accent),
		Dim:       lipgloss.NewStyle().Foreground(p.TextDim),
		OK:        lipgloss.NewStyle().Foreground(p.Success),
		Warn:      lipgloss.NewStyle().Foreground(p.Warning),
		Err:       lipgloss.NewStyle().Foreground(p.Error),
	}
}
