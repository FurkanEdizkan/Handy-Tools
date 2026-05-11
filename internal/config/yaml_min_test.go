package config

import (
	"strings"
	"testing"
)

func TestDecodeMinimalYAMLBasic(t *testing.T) {
	body := strings.Join([]string{
		"theme:",
		"  name: snow",
		"  background: \"#000000\"",
		"mascot:",
		"  enabled: true",
		"  style: wrenly",
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
	if err := decodeMinimalYAML([]byte(body), &c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.Theme.Name != "snow" {
		t.Errorf("theme: %q", c.Theme.Name)
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

func TestDecodeMinimalYAMLIgnoresUnknown(t *testing.T) {
	body := "theme:\n  name: snow\nfuture_key:\n  something: 42\n"
	c := Defaults()
	if err := decodeMinimalYAML([]byte(body), &c); err != nil {
		t.Fatalf("unknown key should be ignored, got: %v", err)
	}
	if c.Theme.Name != "snow" {
		t.Errorf("theme: %q", c.Theme.Name)
	}
}

func TestDecodeMinimalYAMLEmptyListLiteral(t *testing.T) {
	body := "server:\n  allow_roots: []\nrecent: []\n"
	c := Defaults()
	if err := decodeMinimalYAML([]byte(body), &c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(c.Server.AllowRoots) != 0 || len(c.Recent) != 0 {
		t.Errorf("expected empty lists, got server=%v recent=%v", c.Server.AllowRoots, c.Recent)
	}
}

// FuzzDecodeMinimalYAML feeds random byte sequences into the hand-rolled
// parser. The contract is "never panic, return either nil or a structured
// error". This catches index-out-of-range bugs introduced when extending the
// grammar.
func FuzzDecodeMinimalYAML(f *testing.F) {
	seeds := []string{
		"",
		"theme:\n  name: forge\n",
		"server:\n  allow_roots:\n    - /a\n    - /b\n",
		"recent: []\n",
		"# just a comment\n",
		"theme:\n  name: \"with: colon\"\n",
		":::\n",
		"- orphan list item\n",
		"theme:\n\tname: tab-indented\n",
		"image:\n  default_jpeg_quality: not-a-number\n",
		strings.Repeat("a:\n  b: c\n", 200),
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		c := Defaults()
		// Any error is acceptable; a panic is not.
		_ = decodeMinimalYAML(body, &c)
	})
}
