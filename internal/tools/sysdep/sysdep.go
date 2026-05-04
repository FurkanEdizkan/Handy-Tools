// Package sysdep performs runtime detection of the optional system binaries
// Handy uses for features that have no good pure-Go alternative.
//
// Tool packages call sysdep.Lookup(name) at request time and surface a
// structured "MISSING_BINARY" error when a required binary is absent. The
// `handy doctor` command also calls sysdep.All() to enumerate everything.
package sysdep

import (
	"os/exec"
	"sync"
)

// Tool describes one optional external program.
type Tool struct {
	Name        string   // executable name on PATH
	Aliases     []string // accepted alternatives, e.g. unrar / unrar-free
	Description string
	Features    []string // human-readable list of features it unlocks
	InstallHint map[string]string
}

// Known is the canonical list of optional system tools.
var Known = []Tool{
	{
		Name:        "unrar",
		Aliases:     []string{"unrar-free"},
		Description: "Extract RAR archives, including multi-part .partN.rar / .rNN volumes.",
		Features:    []string{"archive: rar extract", "archive: rar multi-part extract"},
		InstallHint: map[string]string{
			"linux":  "apt install unrar  (or unrar-free)",
			"darwin": "brew install unrar",
		},
	},
	{
		Name:        "7z",
		Aliases:     []string{"7zz", "7za"},
		Description: "Extract 7z archives, including multi-part .7z.NNN volumes.",
		Features:    []string{"archive: 7z extract", "archive: 7z multi-part extract"},
		InstallHint: map[string]string{
			"linux":  "apt install p7zip-full",
			"darwin": "brew install p7zip",
		},
	},
	{
		Name:        "pdftoppm",
		Description: "Render PDF pages to PNG/JPEG images.",
		Features:    []string{"pdf: render to image"},
		InstallHint: map[string]string{
			"linux":  "apt install poppler-utils",
			"darwin": "brew install poppler",
		},
	},
	{
		Name:        "pdftotext",
		Description: "Extract plain text from PDF documents.",
		Features:    []string{"pdf: extract text"},
		InstallHint: map[string]string{
			"linux":  "apt install poppler-utils",
			"darwin": "brew install poppler",
		},
	},
	{
		Name:        "magick",
		Aliases:     []string{"convert"},
		Description: "Decode HEIC/HEIF images.",
		Features:    []string{"image: decode HEIC"},
		InstallHint: map[string]string{
			"linux":  "apt install imagemagick",
			"darwin": "brew install imagemagick",
		},
	},
}

// Result is a Tool together with whether it was found and at which path.
type Result struct {
	Tool      Tool
	Found     bool
	Path      string
	UsedAlias string
}

var (
	cacheMu sync.RWMutex
	cache   = map[string]Result{}
)

// Lookup checks whether the named tool (or one of its aliases) is on PATH.
// Results are cached; call Reset() in tests if you mutate PATH.
func Lookup(name string) Result {
	cacheMu.RLock()
	if r, ok := cache[name]; ok {
		cacheMu.RUnlock()
		return r
	}
	cacheMu.RUnlock()

	var t Tool
	for _, k := range Known {
		if k.Name == name {
			t = k
			break
		}
	}
	if t.Name == "" {
		t = Tool{Name: name}
	}

	r := Result{Tool: t}
	candidates := append([]string{t.Name}, t.Aliases...)
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			r.Found = true
			r.Path = p
			r.UsedAlias = c
			break
		}
	}

	cacheMu.Lock()
	cache[name] = r
	cacheMu.Unlock()
	return r
}

// All returns Lookup() results for every Known tool.
func All() []Result {
	out := make([]Result, 0, len(Known))
	for _, t := range Known {
		out = append(out, Lookup(t.Name))
	}
	return out
}

// Reset clears the lookup cache. Tests use this when they manipulate PATH.
func Reset() {
	cacheMu.Lock()
	cache = map[string]Result{}
	cacheMu.Unlock()
}
