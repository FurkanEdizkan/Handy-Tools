package config

import (
	"strings"
	"testing"
)

func TestDecodeYAMLBasic(t *testing.T) {
	body := strings.Join([]string{
		"image:",
		"  default_jpeg_quality: 70",
		"server:",
		"  listen: \":4242\"",
		"  allow_roots:",
		"    - /srv/a",
		"    - /srv/b",
		"recent:",
		"  - /tmp/x",
		"",
	}, "\n")
	c := Defaults()
	if err := decode([]byte(body), &c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.Image.DefaultJPEGQuality != 70 {
		t.Errorf("jpeg quality: %d", c.Image.DefaultJPEGQuality)
	}
	if c.Server.Listen != ":4242" || len(c.Server.AllowRoots) != 2 {
		t.Errorf("server: %+v", c.Server)
	}
	if len(c.Recent) != 1 || c.Recent[0] != "/tmp/x" {
		t.Errorf("recent: %v", c.Recent)
	}
}

func TestDecodeYAMLIgnoresUnknown(t *testing.T) {
	// theme/mascot were removed when the TUI retired; old configs that still
	// carry them must continue to load without error.
	body := "theme:\n  name: snow\nmascot:\n  style: hopper\nfuture_key:\n  something: 42\nimage:\n  default_jpeg_quality: 60\n"
	c := Defaults()
	if err := decode([]byte(body), &c); err != nil {
		t.Fatalf("unknown key should be ignored, got: %v", err)
	}
	if c.Image.DefaultJPEGQuality != 60 {
		t.Errorf("known sibling key not applied: %d", c.Image.DefaultJPEGQuality)
	}
}

// TestDecodeYAMLPartialKeepsDefaults is the round-trip safety contract: a file
// that mentions only one key must leave every other field at its Defaults()
// value.
func TestDecodeYAMLPartialKeepsDefaults(t *testing.T) {
	c := Defaults()
	if err := decode([]byte("image:\n  default_jpeg_quality: 60\n"), &c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.Image.DefaultJPEGQuality != 60 {
		t.Errorf("image quality not applied: %d", c.Image.DefaultJPEGQuality)
	}
	if c.PDF.DefaultDPI != 150 {
		t.Errorf("pdf default lost: %d", c.PDF.DefaultDPI)
	}
	if c.Server.Listen != ":7777" {
		t.Errorf("server default lost: %q", c.Server.Listen)
	}
}

func TestDecodeYAMLEmptyListLiteral(t *testing.T) {
	c := Defaults()
	c.Server.AllowRoots = []string{"/preexisting"}
	c.Recent = []string{"/preexisting"}
	if err := decode([]byte("server:\n  allow_roots: []\nrecent: []\n"), &c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(c.Server.AllowRoots) != 0 || len(c.Recent) != 0 {
		t.Errorf("expected empty lists, got server=%v recent=%v", c.Server.AllowRoots, c.Recent)
	}
}

// FuzzDecodeYAML feeds random byte sequences into the config decoder. The
// contract is "never panic, return either nil or an error" — it guards the
// thin wrapper around gopkg.in/yaml.v3, which has its own upstream fuzzing.
func FuzzDecodeYAML(f *testing.F) {
	seeds := []string{
		"",
		"server:\n  listen: ':4242'\n",
		"server:\n  allow_roots:\n    - /a\n    - /b\n",
		"recent: []\n",
		"# just a comment\n",
		"image:\n  default_jpeg_quality: 60\n",
		":::\n",
		"- orphan list item\n",
		"server:\n\tlisten: tab-indented\n",
		"image:\n  default_jpeg_quality: not-a-number\n",
		strings.Repeat("a:\n  b: c\n", 200),
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		c := Defaults()
		// Any error is acceptable; a panic is not.
		_ = decode(body, &c)
	})
}
