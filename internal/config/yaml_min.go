package config

import (
	"fmt"
	"strconv"
	"strings"
)

// decodeMinimalYAML is a deliberately tiny YAML decoder targeting Handy's
// flat config schema: top-level mapping keys with one level of nesting,
// scalar values, and short list-of-strings under `allow_roots:` and `recent:`.
//
// It is NOT a general YAML implementation. We accept this in exchange for
// keeping the Phase 5 commit dependency-free; we can swap in gopkg.in/yaml.v3
// later when we add it for other reasons.
func decodeMinimalYAML(body []byte, cfg *Config) error {
	var section string
	var listTarget *[]string

	lines := strings.Split(string(body), "\n")
	for ln, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		// list element: matches "  - value" or "    - value" depending on section
		if t := strings.TrimLeft(line, " "); strings.HasPrefix(t, "- ") {
			if listTarget == nil {
				return fmt.Errorf("line %d: unexpected list item", ln+1)
			}
			val := strings.TrimSpace(strings.TrimPrefix(t, "-"))
			val = strings.Trim(val, `"`)
			*listTarget = append(*listTarget, val)
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)

		if indent == 0 {
			// top-level key
			key := strings.TrimSuffix(trimmed, ":")
			section = key
			listTarget = nil
			// inline list literal `recent: []` etc.
			if strings.HasSuffix(trimmed, "[]") {
				switch key {
				case "recent":
					cfg.Recent = nil
				}
				section = ""
			}
			continue
		}

		// nested key inside section (indent == 2)
		key, val, ok := splitKV(trimmed)
		if !ok {
			return fmt.Errorf("line %d: %q: expected key: value", ln+1, raw)
		}
		if val == "" {
			// nested list opener like "allow_roots:"
			switch section + "." + key {
			case "server.allow_roots":
				cfg.Server.AllowRoots = nil
				listTarget = &cfg.Server.AllowRoots
			default:
				listTarget = nil
			}
			if strings.HasSuffix(trimmed, "[]") {
				listTarget = nil
			}
			continue
		}
		listTarget = nil
		if err := assign(cfg, section, key, val); err != nil {
			return fmt.Errorf("line %d: %w", ln+1, err)
		}
	}
	return nil
}

func splitKV(s string) (string, string, bool) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "", "", false
	}
	k := strings.TrimSpace(s[:i])
	v := strings.TrimSpace(s[i+1:])
	v = strings.Trim(v, `"`)
	return k, v, true
}

func assign(cfg *Config, section, key, val string) error {
	switch section + "." + key {
	case "theme.name":
		cfg.Theme.Name = val
	case "theme.background":
		cfg.Theme.Background = val
	case "theme.accent":
		cfg.Theme.Accent = val
	case "mascot.enabled":
		cfg.Mascot.Enabled = parseBool(val)
	case "mascot.style":
		cfg.Mascot.Style = val
	case "archive.auto_extract_multi_part":
		cfg.Archive.AutoExtractMultiPart = parseBool(val)
	case "archive.overwrite_by_default":
		cfg.Archive.OverwriteByDefault = parseBool(val)
	case "archive.default_destination":
		cfg.Archive.DefaultDestination = val
	case "image.default_jpeg_quality":
		n, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		cfg.Image.DefaultJPEGQuality = n
	case "image.strip_metadata":
		cfg.Image.StripMetadata = parseBool(val)
	case "pdf.default_dpi":
		n, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		cfg.PDF.DefaultDPI = n
	case "server.listen":
		cfg.Server.Listen = val
	default:
		// silently ignore unknown keys to keep config forward-compatible
	}
	return nil
}

func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}
