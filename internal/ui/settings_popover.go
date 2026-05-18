package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
)

// settingsPopover is the top-left overlay anchored to the cog button — the
// design's SettingsPop component from the htools-tui.html prototype. It
// replaces the standalone Settings page so theme / mascot / scanlines /
// htoolsd toggles are reachable without leaving the home or tool view.
//
// Scanlines and the htoolsd target are local UI state only — config.Config
// has no fields for them today. The chips are still wired so the design's
// affordance is preserved.
type settingsPopover struct {
	open      bool
	cursor    int    // focused row, 0..popRowCount-1
	scanlines bool   // local; not persisted
	daemon    string // off / local / remote — local; not persisted
}

const popRowCount = 4

func newSettingsPopover() settingsPopover {
	return settingsPopover{scanlines: true, daemon: "off"}
}

// Toggle flips the open state and resets the focused row when closing.
func (p *settingsPopover) Toggle() {
	p.open = !p.open
	if !p.open {
		p.cursor = 0
	}
}

// Close hides the popover.
func (p *settingsPopover) Close() {
	p.open = false
	p.cursor = 0
}

// IsOpen reports whether the popover should be rendered.
func (p *settingsPopover) IsOpen() bool { return p.open }

// MoveUp / MoveDown navigate between rows.
func (p *settingsPopover) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}
func (p *settingsPopover) MoveDown() {
	if p.cursor < popRowCount-1 {
		p.cursor++
	}
}

// Cycle advances the focused row's value forward. Returns true if anything
// shared (theme, mascot) changed so the router can rebuild styles.
func (p *settingsPopover) Cycle(cfg *config.Config) bool {
	switch p.cursor {
	case 0:
		cfg.Theme.Name = nextTheme(cfg.Theme.Name)
		return true
	case 1:
		p.scanlines = !p.scanlines
		return true
	case 2:
		cfg.Mascot.Style = cycleMascotCharacter(cfg.Mascot.Style)
		return true
	case 3:
		p.daemon = nextDaemon(p.daemon)
		return true
	}
	return false
}

func nextTheme(t string) string {
	switch t {
	case "forge":
		return "snow"
	case "snow":
		return "ember"
	}
	return "forge"
}

// mascotCharacterName renders the on-disk cfg.Mascot.Style as a user-facing
// character name. Legacy "classic" maps to "wrenly".
func mascotCharacterName(style string) string {
	if style == "hopper" {
		return "hopper"
	}
	return "wrenly"
}

// cycleMascotCharacter advances wrenly → hopper → wrenly.
func cycleMascotCharacter(style string) string {
	if style == "hopper" {
		return "wrenly"
	}
	return "hopper"
}

func nextDaemon(d string) string {
	switch d {
	case "off":
		return "local"
	case "local":
		return "remote"
	}
	return "off"
}

// View renders the popover card. width is the card's interior width in cells.
// The caller is responsible for placing the result in the surrounding frame.
func (p *settingsPopover) View(s theme.Styles, cfg config.Config, width int) string {
	if width < 24 {
		width = 24
	}

	title := s.Accent.Bold(true).Render("SETTINGS")
	hint := s.Dim.Render("esc")
	headPad := width - lipgloss.Width(title) - lipgloss.Width(hint) - 2
	if headPad < 1 {
		headPad = 1
	}
	header := title + strings.Repeat(" ", headPad) + hint

	rows := []string{
		header,
		s.Dim.Render(strings.Repeat("─", width-2)),
	}

	scanl := "off"
	if p.scanlines {
		scanl = "on"
	}

	rows = append(rows,
		p.renderRow(s, 0, "Theme", []string{"forge", "snow", "ember"}, cfg.Theme.Name),
		p.renderRow(s, 1, "Scanlines", []string{"on", "off"}, scanl),
		p.renderRow(s, 2, "Mascot", []string{"wrenly", "hopper"}, mascotCharacterName(cfg.Mascot.Style)),
		p.renderRow(s, 3, "htoolsd", []string{"off", "local", "remote"}, p.daemon),
		"",
		s.Dim.Render("~/.config/handy-tools/config.yaml"),
	)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.P.Accent).
		Background(s.P.Surface).
		Foreground(s.P.Text).
		Padding(0, 1).
		Width(width)
	return card.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderRow draws one labelled row of chips, optionally with the focus arrow.
func (p *settingsPopover) renderRow(s theme.Styles, idx int, label string, choices []string, current string) string {
	chipStyle := lipgloss.NewStyle().
		Foreground(s.P.TextDim).
		Background(s.P.Background).
		Border(lipgloss.NormalBorder()).
		BorderForeground(s.P.Border).
		Padding(0, 1)
	activeChip := chipStyle.
		Foreground(s.P.Accent).
		BorderForeground(s.P.Accent).
		Bold(true)

	parts := make([]string, 0, len(choices))
	for _, c := range choices {
		style := chipStyle
		if c == current {
			style = activeChip
		}
		parts = append(parts, style.Render(c))
	}
	chips := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	caret := "  "
	if idx == p.cursor {
		caret = s.Accent.Bold(true).Render("▸ ")
	}
	lbl := s.Dim.Render(label)
	return lipgloss.JoinHorizontal(lipgloss.Top, caret, lipgloss.NewStyle().Width(12).Render(lbl), chips)
}
