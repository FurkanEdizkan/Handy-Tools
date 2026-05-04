package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy/internal/ui/theme"
)

type fileEntry struct {
	name  string
	isDir bool
	size  int64
	path  string
}

type filesPage struct {
	styles  theme.Styles
	cwd     string
	entries []fileEntry
	cursor  int
	status  string
}

func newFilesPage(s theme.Styles) Page {
	wd, _ := os.Getwd()
	p := &filesPage{styles: s, cwd: wd}
	p.refresh()
	return p
}

func (f *filesPage) Init() tea.Cmd { return nil }
func (f *filesPage) ID() PageID    { return PageFiles }
func (f *filesPage) Title() string { return "Files - " + f.cwd }

func (f *filesPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			if f.cursor > 0 {
				f.cursor--
			}
		case "down", "j":
			if f.cursor < len(f.entries)-1 {
				f.cursor++
			}
		case "enter":
			return f, f.activate()
		case "h", "left":
			f.cwd = filepath.Dir(f.cwd)
			f.cursor = 0
			f.refresh()
		}
	}
	return f, nil
}

func (f *filesPage) activate() tea.Cmd {
	if len(f.entries) == 0 {
		return nil
	}
	e := f.entries[f.cursor]
	if e.isDir {
		f.cwd = e.path
		f.cursor = 0
		f.refresh()
		return nil
	}
	actions := actionsFor(e.path)
	if len(actions) == 0 {
		f.status = "No actions available for " + e.name
		return func() tea.Msg {
			return MascotMsg{State: 0, Greeting: "Pick another file."}
		}
	}
	f.status = "Would run: " + strings.Join(actions, ", ")
	return func() tea.Msg {
		return MascotMsg{State: 1, Greeting: "Thinking about " + e.name + "..."}
	}
}

func (f *filesPage) refresh() {
	dir, err := os.ReadDir(f.cwd)
	if err != nil {
		f.entries = nil
		f.status = err.Error()
		return
	}
	out := make([]fileEntry, 0, len(dir))
	for _, d := range dir {
		info, err := d.Info()
		if err != nil {
			continue
		}
		out = append(out, fileEntry{
			name:  d.Name(),
			isDir: d.IsDir(),
			size:  info.Size(),
			path:  filepath.Join(f.cwd, d.Name()),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].isDir != out[j].isDir {
			return out[i].isDir
		}
		return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
	})
	f.entries = out
	f.status = ""
}

func (f *filesPage) View() string {
	rows := make([]string, 0, len(f.entries)+2)
	for i, e := range f.entries {
		marker := "  "
		if i == f.cursor {
			marker = "> "
		}
		name := e.name
		if e.isDir {
			name += "/"
		}
		line := marker + name
		if i == f.cursor {
			line = lipgloss.NewStyle().Foreground(f.styles.P.Accent).Render(line)
		}
		rows = append(rows, line)
	}
	if f.status != "" {
		rows = append(rows, f.styles.Status.Render(f.status))
	}
	rows = append(rows, f.styles.Status.Render("up/down  -  enter to open  -  left to go up  -  tab next page"))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// actionsFor returns the human-readable actions that apply to a path. The
// real wiring lives in the tool packages; this is just for routing.
func actionsFor(path string) []string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff", ".webp", ".heic":
		return []string{"convert image"}
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".tgz", ".tbz2":
		return []string{"inspect archive", "extract archive"}
	case ".pdf":
		return []string{"pdf to image", "pdf to text", "pdf split"}
	}
	return nil
}
