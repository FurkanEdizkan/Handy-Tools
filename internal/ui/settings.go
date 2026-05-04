package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy/internal/config"
	"github.com/furkandedizkan/handy/internal/ui/theme"
)

type settingsField struct {
	label string
	get   func(*config.Config) string
	cycle func(*config.Config)
}

type settingsPage struct {
	styles theme.Styles
	cfg    *config.Config
	cursor int
	fields []settingsField
	status string
}

func newSettingsPage(s theme.Styles, cfg *config.Config) Page {
	p := &settingsPage{styles: s, cfg: cfg}
	p.fields = []settingsField{
		{
			label: "Theme",
			get:   func(c *config.Config) string { return c.Theme.Name },
			cycle: func(c *config.Config) {
				if c.Theme.Name == "snow" {
					c.Theme.Name = "ember"
				} else {
					c.Theme.Name = "snow"
				}
			},
		},
		{
			label: "Mascot",
			get:   func(c *config.Config) string { return boolWord(c.Mascot.Enabled) },
			cycle: func(c *config.Config) { c.Mascot.Enabled = !c.Mascot.Enabled },
		},
		{
			label: "Auto-extract multi-part",
			get:   func(c *config.Config) string { return boolWord(c.Archive.AutoExtractMultiPart) },
			cycle: func(c *config.Config) { c.Archive.AutoExtractMultiPart = !c.Archive.AutoExtractMultiPart },
		},
		{
			label: "Overwrite by default",
			get:   func(c *config.Config) string { return boolWord(c.Archive.OverwriteByDefault) },
			cycle: func(c *config.Config) { c.Archive.OverwriteByDefault = !c.Archive.OverwriteByDefault },
		},
		{
			label: "JPEG quality",
			get:   func(c *config.Config) string { return fmt.Sprintf("%d", c.Image.DefaultJPEGQuality) },
			cycle: func(c *config.Config) {
				c.Image.DefaultJPEGQuality += 5
				if c.Image.DefaultJPEGQuality > 100 {
					c.Image.DefaultJPEGQuality = 50
				}
			},
		},
		{
			label: "PDF render DPI",
			get:   func(c *config.Config) string { return fmt.Sprintf("%d", c.PDF.DefaultDPI) },
			cycle: func(c *config.Config) {
				c.PDF.DefaultDPI += 50
				if c.PDF.DefaultDPI > 400 {
					c.PDF.DefaultDPI = 100
				}
			},
		},
	}
	return p
}

func (s *settingsPage) Init() tea.Cmd { return nil }
func (s *settingsPage) ID() PageID    { return PageSettings }
func (s *settingsPage) Title() string { return "Settings" }

func (s *settingsPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.fields)-1 {
				s.cursor++
			}
		case "enter", " ", "right", "l":
			s.fields[s.cursor].cycle(s.cfg)
		case "s":
			path, err := config.Save(*s.cfg)
			if err != nil {
				s.status = "Save failed: " + err.Error()
			} else {
				s.status = "Saved to " + path
			}
		}
	}
	return s, nil
}

func (s *settingsPage) View() string {
	rows := []string{}
	for i, f := range s.fields {
		line := fmt.Sprintf("  %-26s  %s", f.label, f.get(s.cfg))
		if i == s.cursor {
			line = lipgloss.NewStyle().Foreground(s.styles.P.Accent).Render("> " + line[2:])
		}
		rows = append(rows, line)
	}
	if s.status != "" {
		rows = append(rows, "", s.styles.Status.Render(s.status))
	}
	rows = append(rows, "", s.styles.Status.Render("up/down to move  -  enter to cycle  -  s to save"))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func boolWord(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
