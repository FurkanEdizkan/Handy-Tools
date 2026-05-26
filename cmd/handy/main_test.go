package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRoute(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantDec     decision
		wantBackend string
		wantArgs    []string
		wantBadVerb string
	}{
		{"bare opens GUI", nil, decExecBackend, "htools-gui", nil, ""},
		{"gui verb opens GUI explicitly", []string{"gui"}, decExecBackend, "htools-gui", []string{}, ""},
		{"gui verb forwards trailing args", []string{"gui", "--foo"}, decExecBackend, "htools-gui", []string{"--foo"}, ""},
		{"serve routes to htoolsd", []string{"serve", "--listen", ":7777"}, decExecBackend, "htoolsd", []string{"--listen", ":7777"}, ""},
		{"daemon alias", []string{"daemon"}, decExecBackend, "htoolsd", []string{}, ""},
		{"mcp routes to htools-mcp", []string{"mcp"}, decExecBackend, "htools-mcp", []string{}, ""},
		{"mcp forwards flags", []string{"mcp", "--allow-roots", "/tmp"}, decExecBackend, "htools-mcp", []string{"--allow-roots", "/tmp"}, ""},
		{"convert forwards verb + args", []string{"convert", "in.png", "--format", "jpeg"}, decExecBackend, "htools", []string{"convert", "in.png", "--format", "jpeg"}, ""},
		{"doctor forwards verb alone", []string{"doctor"}, decExecBackend, "htools", []string{"doctor"}, ""},
		{"pdf forwards subverb + args", []string{"pdf", "merge", "a.pdf", "b.pdf"}, decExecBackend, "htools", []string{"pdf", "merge", "a.pdf", "b.pdf"}, ""},
		{"diff-tree verb", []string{"diff-tree", "a", "b"}, decExecBackend, "htools", []string{"diff-tree", "a", "b"}, ""},
		{"strip-meta verb", []string{"strip-meta", "x.jpg"}, decExecBackend, "htools", []string{"strip-meta", "x.jpg"}, ""},

		{"version verb", []string{"version"}, decPrintVersion, "", nil, ""},
		{"--version flag", []string{"--version"}, decPrintVersion, "", nil, ""},
		{"-v short flag", []string{"-v"}, decPrintVersion, "", nil, ""},
		{"help verb", []string{"help"}, decPrintHelp, "", nil, ""},
		{"--help flag", []string{"--help"}, decPrintHelp, "", nil, ""},
		{"-h short flag", []string{"-h"}, decPrintHelp, "", nil, ""},

		{"unknown subcommand", []string{"frobnicate"}, decUnknown, "", nil, "frobnicate"},
		{"unknown leading flag", []string{"--bogus"}, decUnknown, "", nil, "--bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := route(tt.args)
			if got.Decision != tt.wantDec {
				t.Errorf("Decision: got %d, want %d", got.Decision, tt.wantDec)
			}
			if got.Backend != tt.wantBackend {
				t.Errorf("Backend: got %q, want %q", got.Backend, tt.wantBackend)
			}
			if !equalArgs(got.Args, tt.wantArgs) {
				t.Errorf("Args: got %#v, want %#v", got.Args, tt.wantArgs)
			}
			if got.BadVerb != tt.wantBadVerb {
				t.Errorf("BadVerb: got %q, want %q", got.BadVerb, tt.wantBadVerb)
			}
		})
	}
}

func TestRunPrintsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--version"}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit: got %d, want 0", exit)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected version printed to stdout")
	}
}

func TestRunPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--help"}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit: got %d, want 0", exit)
	}
	out := stdout.String()
	// Spot-check the help text mentions the main verbs.
	for _, want := range []string{"convert", "serve", "mcp", "gui", "--version"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q; got:\n%s", want, out)
		}
	}
}

func TestRunUnknownVerbExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run([]string{"frobnicate"}, &stdout, &stderr)
	if exit != 2 {
		t.Fatalf("exit: got %d, want 2", exit)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Errorf("stderr missing 'unknown subcommand'; got:\n%s", stderr.String())
	}
}

func TestBackendNotFoundHintGUIIncludesBuildPointers(t *testing.T) {
	got := backendNotFoundHint("htools-gui", errors.New("ignored"))
	// The GUI-specific hint must surface both the install.sh path and the
	// dev-build path — a future "clean up the wording" pass could easily
	// drop one and silently regress the UX.
	for _, want := range []string{
		"htools-gui",
		"install.sh",
		"make gui-build",
		"libwebkit2gtk",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("htools-gui hint missing %q; got:\n%s", want, got)
		}
	}
}

func TestBackendNotFoundHintGenericForOtherBackends(t *testing.T) {
	for _, backend := range []string{"htools", "htoolsd", "htools-mcp"} {
		err := errors.New("couldn't find " + backend + " on PATH")
		got := backendNotFoundHint(backend, err)
		if !strings.Contains(got, "handy:") {
			t.Errorf("%s hint missing handy: prefix; got: %q", backend, got)
		}
		if strings.Contains(got, "make gui-build") {
			t.Errorf("%s hint leaked GUI-specific text; got: %q", backend, got)
		}
		if !strings.Contains(got, err.Error()) {
			t.Errorf("%s hint should wrap the underlying error; got: %q", backend, got)
		}
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
