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

	"gopkg.in/yaml.v3"
)

// Config is the persisted user settings tree.
type Config struct {
	Theme   Theme    `yaml:"theme"    json:"theme"`
	Mascot  Mascot   `yaml:"mascot"   json:"mascot"`
	Archive Archive  `yaml:"archive"  json:"archive"`
	Image   Image    `yaml:"image"    json:"image"`
	PDF     PDF      `yaml:"pdf"      json:"pdf"`
	Server  Server   `yaml:"server"   json:"server"`
	Recent  []string `yaml:"recent"   json:"recent"`
}

type Theme struct {
	Name       string `yaml:"name"       json:"name"`       // "forge" (default), "snow", "ember"
	Background string `yaml:"background" json:"background"` // hex; empty -> theme default
	Accent     string `yaml:"accent"     json:"accent"`
}

// Mascot configures the companion sprite. Style selects which character is
// drawn — it is not a rendering-detail switch (there is no "minimal"/"full"
// variant): the two values are the two characters, "wrenly" (the orange panda,
// default) and "hopper" (the lilac rabbit). The legacy value "classic" and any
// unrecognized value are treated as "wrenly" by the UI, so old configs keep
// working.
type Mascot struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Style   string `yaml:"style"   json:"style"` // "wrenly" (default) | "hopper"
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
	Listen     string   `yaml:"listen"      json:"listen"`      // gRPC, ":7777"
	HTTPListen string   `yaml:"http_listen" json:"http_listen"` // HTTP/SSE, e.g. ":8080"; empty -> disabled
	AllowRoots []string `yaml:"allow_roots" json:"allow_roots"` // empty -> CWD only
}

// Defaults returns a freshly-populated Config.
func Defaults() Config {
	return Config{
		Theme:  Theme{Name: "forge"},
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

// loadFile reads and decodes the YAML config at path. It starts from
// Defaults() and unmarshals over it, so a partial file leaves every absent
// field at its default value.
func loadFile(path string) (Config, error) {
	cfg := Defaults()
	body, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return cfg, err
	}
	if err := decode(body, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// decode unmarshals YAML into cfg. Keys absent from the document keep their
// existing values, which is what lets partial config files round-trip safely.
// Unknown keys are ignored for forward compatibility.
func decode(body []byte, cfg *Config) error {
	return yaml.Unmarshal(body, cfg)
}

// writeYAML serializes c as YAML. Field order and key names come from the
// `yaml:` struct tags on Config.
func writeYAML(w io.Writer, c Config) error {
	body, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}
