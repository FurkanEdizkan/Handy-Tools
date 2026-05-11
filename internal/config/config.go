// Package config loads and persists user settings.
//
// Lookup order:
//  1. $HANDY_TOOLS_CONFIG (if set, treated as a file path)
//  2. $XDG_CONFIG_HOME/handy-tools/config.yaml
//  3. ~/.config/handy-tools/config.yaml
//
// The on-disk format is YAML with stable keys. Defaults() returns a complete
// Config so we can safely round-trip even partial files.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Config is the persisted user settings tree.
type Config struct {
	Theme    Theme    `yaml:"theme"    json:"theme"`
	Mascot   Mascot   `yaml:"mascot"   json:"mascot"`
	Archive  Archive  `yaml:"archive"  json:"archive"`
	Image    Image    `yaml:"image"    json:"image"`
	PDF      PDF      `yaml:"pdf"      json:"pdf"`
	Server   Server   `yaml:"server"   json:"server"`
	Recent   []string `yaml:"recent"   json:"recent"`
}

type Theme struct {
	Name       string `yaml:"name"       json:"name"`        // "forge" (default), "snow", "ember"
	Background string `yaml:"background" json:"background"`  // hex; empty -> theme default
	Accent     string `yaml:"accent"     json:"accent"`
}

type Mascot struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Style   string `yaml:"style"   json:"style"`   // "wrenly" (default)
}

type Archive struct {
	AutoExtractMultiPart bool   `yaml:"auto_extract_multi_part" json:"auto_extract_multi_part"`
	OverwriteByDefault   bool   `yaml:"overwrite_by_default"    json:"overwrite_by_default"`
	DefaultDestination   string `yaml:"default_destination"     json:"default_destination"` // "" -> alongside source
}

type Image struct {
	DefaultJPEGQuality int  `yaml:"default_jpeg_quality" json:"default_jpeg_quality"`
	StripMetadata      bool `yaml:"strip_metadata"       json:"strip_metadata"`
}

type PDF struct {
	DefaultDPI int `yaml:"default_dpi" json:"default_dpi"`
}

type Server struct {
	Listen     string   `yaml:"listen"      json:"listen"`     // ":7777"
	AllowRoots []string `yaml:"allow_roots" json:"allow_roots"` // empty -> CWD only
}

// Defaults returns a freshly-populated Config.
func Defaults() Config {
	return Config{
		Theme: Theme{Name: "forge"},
		Mascot: Mascot{Enabled: true, Style: "wrenly"},
		Archive: Archive{
			AutoExtractMultiPart: false,
			OverwriteByDefault:   false,
		},
		Image:  Image{DefaultJPEGQuality: 90},
		PDF:    PDF{DefaultDPI: 150},
		Server: Server{Listen: ":7777"},
	}
}

// Path returns the resolved config file path according to the lookup order.
func Path() (string, error) {
	if v := os.Getenv("HANDY_TOOLS_CONFIG"); v != "" {
		return v, nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "handy-tools", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "handy-tools", "config.yaml"), nil
}

// Load reads the config from disk. Missing file -> Defaults() with no error.
func Load() (Config, string, error) {
	path, err := Path()
	if err != nil {
		return Defaults(), "", err
	}
	cfg, err := loadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), path, nil
	}
	return cfg, path, err
}

// Save writes c to disk, creating the directory if needed.
func Save(c Config) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".handy-tools-config-*.yaml")
	if err != nil {
		return path, err
	}
	defer os.Remove(tmp.Name())
	if err := writeYAML(tmp, c); err != nil {
		tmp.Close()
		return path, err
	}
	if err := tmp.Close(); err != nil {
		return path, err
	}
	return path, os.Rename(tmp.Name(), path)
}

// loadFile reads YAML using a tiny hand-rolled parser to avoid pulling in
// a yaml dependency for Phase 5. We only need flat scalar keys plus the
// `recent:` list, which is enough until we adopt a real yaml lib.
func loadFile(path string) (Config, error) {
	cfg := Defaults()
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return cfg, err
	}
	if err := decodeMinimalYAML(body, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// writeYAML emits the tiny YAML dialect that decodeMinimalYAML can read.
func writeYAML(w io.Writer, c Config) error {
	out := &strings.Builder{}
	fmt.Fprintf(out, "theme:\n  name: %s\n  background: %s\n  accent: %s\n",
		yamlString(c.Theme.Name), yamlString(c.Theme.Background), yamlString(c.Theme.Accent))
	fmt.Fprintf(out, "mascot:\n  enabled: %t\n  style: %s\n",
		c.Mascot.Enabled, yamlString(c.Mascot.Style))
	fmt.Fprintf(out, "archive:\n  auto_extract_multi_part: %t\n  overwrite_by_default: %t\n  default_destination: %s\n",
		c.Archive.AutoExtractMultiPart, c.Archive.OverwriteByDefault, yamlString(c.Archive.DefaultDestination))
	fmt.Fprintf(out, "image:\n  default_jpeg_quality: %d\n  strip_metadata: %t\n",
		c.Image.DefaultJPEGQuality, c.Image.StripMetadata)
	fmt.Fprintf(out, "pdf:\n  default_dpi: %d\n", c.PDF.DefaultDPI)
	fmt.Fprintf(out, "server:\n  listen: %s\n", yamlString(c.Server.Listen))
	if len(c.Server.AllowRoots) > 0 {
		out.WriteString("  allow_roots:\n")
		for _, r := range c.Server.AllowRoots {
			fmt.Fprintf(out, "    - %s\n", yamlString(r))
		}
	} else {
		out.WriteString("  allow_roots: []\n")
	}
	if len(c.Recent) > 0 {
		out.WriteString("recent:\n")
		for _, r := range c.Recent {
			fmt.Fprintf(out, "  - %s\n", yamlString(r))
		}
	} else {
		out.WriteString("recent: []\n")
	}
	_, err := io.WriteString(w, out.String())
	return err
}

func yamlString(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#\n\"") {
		// crude but sufficient quoting
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
