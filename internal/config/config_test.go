package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultsHaveSensibleValues(t *testing.T) {
	c := Defaults()
	if c.Theme.Name != "snow" {
		t.Fatalf("theme name: got %q want snow", c.Theme.Name)
	}
	if !c.Mascot.Enabled || c.Mascot.Style != "snow-fox" {
		t.Fatalf("mascot defaults wrong: %+v", c.Mascot)
	}
	if c.Image.DefaultJPEGQuality != 90 {
		t.Fatalf("jpeg quality default: %d", c.Image.DefaultJPEGQuality)
	}
	if c.PDF.DefaultDPI != 150 {
		t.Fatalf("pdf dpi default: %d", c.PDF.DefaultDPI)
	}
	if c.Server.Listen != ":7777" {
		t.Fatalf("server listen default: %q", c.Server.Listen)
	}
}

func TestPathRespectsHandyConfigEnv(t *testing.T) {
	t.Setenv("HANDY_CONFIG", "/tmp/handy.yaml")
	p, err := Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if p != "/tmp/handy.yaml" {
		t.Fatalf("path: got %q", p)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HANDY_CONFIG", filepath.Join(dir, "handy.yaml"))

	c := Defaults()
	c.Theme.Name = "ember"
	c.Archive.AutoExtractMultiPart = true
	c.Image.DefaultJPEGQuality = 75
	c.Server.Listen = ":9000"
	c.Server.AllowRoots = []string{"/srv/uploads", "/srv/downloads"}
	c.Recent = []string{"/tmp/a.zip", "/tmp/b.pdf"}

	if _, err := Save(c); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, _, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Theme.Name != "ember" {
		t.Errorf("theme: %+v", got.Theme)
	}
	if !got.Archive.AutoExtractMultiPart {
		t.Errorf("archive: %+v", got.Archive)
	}
	if got.Image.DefaultJPEGQuality != 75 {
		t.Errorf("image quality: %d", got.Image.DefaultJPEGQuality)
	}
	if got.Server.Listen != ":9000" || len(got.Server.AllowRoots) != 2 {
		t.Errorf("server: %+v", got.Server)
	}
	if len(got.Recent) != 2 {
		t.Errorf("recent: %v", got.Recent)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HANDY_CONFIG", filepath.Join(dir, "does-not-exist.yaml"))

	c, _, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Theme.Name != "snow" {
		t.Fatalf("expected defaults, got %+v", c)
	}
}
