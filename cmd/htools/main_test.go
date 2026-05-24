package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/config"
)

func TestRunUnknownCommand(t *testing.T) {
	if code := run([]string{"nope"}); code != 2 {
		t.Errorf("unknown verb exit = %d, want 2", code)
	}
}

func TestRunNoArgsShowsUsage(t *testing.T) {
	if code := run(nil); code != 2 {
		t.Errorf("no-args exit = %d, want 2", code)
	}
}

func TestRunVersion(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	for _, arg := range []string{"version", "--version", "-v"} {
		if code := run([]string{arg}); code != 0 {
			t.Errorf("%s exit = %d, want 0", arg, code)
		}
	}
}

func TestRunHelp(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	for _, arg := range []string{"help", "--help", "-h"} {
		if code := run([]string{arg}); code != 0 {
			t.Errorf("%s exit = %d, want 0", arg, code)
		}
	}
}

// TestRunDoctor exercises the doctor path through the dispatch surface so
// any future re-wiring keeps the no-arg path alive.
func TestRunDoctor(t *testing.T) {
	t.Setenv("HANDY_TOOLS_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	if code := run([]string{"doctor"}); code != 0 {
		t.Errorf("doctor exit = %d, want 0", code)
	}
}

// TestDispatchMissingFlags confirms each subcommand surfaces a non-zero exit
// when its required flags are missing, without needing real fixtures.
func TestDispatchMissingFlags(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	cases := []struct {
		name string
		verb string
		args []string
	}{
		{"convert no sources", "convert", []string{"--format", "jpeg"}},
		{"convert no format", "convert", []string{"in.png"}},
		{"convert bad format", "convert", []string{"--format", "galaxy", "in.png"}},
		{"pack no sources", "pack", []string{"--format", "zip", "--output", "out.zip"}},
		{"pack no format", "pack", []string{"--output", "out.zip", "in.png"}},
		{"pack no output", "pack", []string{"--format", "zip", "in.png"}},
		{"pack bad format", "pack", []string{"--format", "rar", "--output", "out.rar", "in.png"}},
		{"extract no source", "extract", []string{}},
		{"extract too many", "extract", []string{"a.zip", "b.zip"}},
		{"pdf no op", "pdf", []string{}},
		{"pdf bad op", "pdf", []string{"shred"}},
		{"pdf merge too few", "pdf", []string{"merge", "--out", "m.pdf", "a.pdf"}},
		{"pdf merge no out", "pdf", []string{"merge", "a.pdf", "b.pdf"}},
		{"pdf split no out", "pdf", []string{"split", "--pages", "1-5", "in.pdf"}},
		{"pdf split both modes", "pdf", []string{"split", "--pages", "1-5", "--every", "2", "--out", "d", "in.pdf"}},
		{"pdf split neither mode", "pdf", []string{"split", "--out", "d", "in.pdf"}},
		{"pdf render no out", "pdf", []string{"render", "in.pdf"}},
		{"pdf text no source", "pdf", []string{"text"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := dispatch(ctx, cfg, tc.verb, tc.args); code == 0 {
				t.Errorf("expected non-zero exit, got 0")
			}
		})
	}
}

func TestParseImageFormatRoundTrip(t *testing.T) {
	wantPairs := map[string]bool{
		"jpeg": true, "jpg": true, "png": true, "webp": true,
		"gif": true, "bmp": true, "tiff": true, "tif": true,
		"heic": true, "heif": true,
	}
	for s := range wantPairs {
		if _, ok := parseImageFormat(s); !ok {
			t.Errorf("parseImageFormat(%q) = false; want true", s)
		}
	}
	if _, ok := parseImageFormat("galaxy"); ok {
		t.Error("parseImageFormat(galaxy) = true; want false")
	}
}

func TestParseArchiveFormatRoundTrip(t *testing.T) {
	want := []string{"zip", "tar", "tar.gz", "tgz", "tar.bz2", "tbz", "tbz2", "tar.zst", "tzst", "zst", "7z"}
	for _, s := range want {
		if _, ok := parseArchiveFormat(s); !ok {
			t.Errorf("parseArchiveFormat(%q) = false; want true", s)
		}
	}
	if _, ok := parseArchiveFormat("rar"); ok {
		t.Error("parseArchiveFormat(rar) = true; rar pack should not be supported")
	}
}

func TestParseRange(t *testing.T) {
	cases := []struct {
		in        string
		wantFrom  int
		wantTo    int
		wantError bool
	}{
		{"1-5", 1, 5, false},
		{"3-", 3, 0, false},
		{"-5", 0, 5, false},
		{"4", 4, 4, false},
		{"", 0, 0, true},
		{"abc", 0, 0, true},
		{"5-3", 0, 0, true},
		{"0-2", 0, 0, true}, // FROM must be ≥1
	}
	for _, tc := range cases {
		r, err := parseRange(tc.in)
		if tc.wantError {
			if err == nil {
				t.Errorf("parseRange(%q) = %+v, want error", tc.in, r)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseRange(%q) errored: %v", tc.in, err)
			continue
		}
		if r.From != tc.wantFrom || r.To != tc.wantTo {
			t.Errorf("parseRange(%q) = {%d, %d}; want {%d, %d}", tc.in, r.From, r.To, tc.wantFrom, tc.wantTo)
		}
	}
}
