package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/config"
)

func TestCmdDiffTreeUsageErrors(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	cases := []struct {
		name string
		args []string
	}{
		{"no paths", []string{}},
		{"one path", []string{"/tmp/a"}},
		{"three paths", []string{"/tmp/a", "/tmp/b", "/tmp/c"}},
		{"bad --by", []string{"--by", "galaxy", "/tmp/a", "/tmp/b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := cmdDiffTree(ctx, cfg, tc.args); code == 0 {
				t.Errorf("expected non-zero exit for %s", tc.name)
			}
		})
	}
}

func TestCmdDiffTreeNoDiffsExitZero(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	out := captureStdout(t, func() {
		if code := cmdDiffTree(context.Background(), config.Defaults(),
			[]string{"--quiet", a, b}); code != 0 {
			t.Fatalf("empty trees should match (exit 0), got %d", code)
		}
	})
	if out != "" {
		t.Errorf("expected no diff lines on empty trees, got %q", out)
	}
}

func TestCmdDiffTreeDifferingExitOne(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if err := os.WriteFile(filepath.Join(a, "only_a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := captureStdout(t, func() {
		if code := cmdDiffTree(context.Background(), config.Defaults(),
			[]string{"--quiet", a, b}); code != 1 {
			t.Fatalf("differing trees exit = %d, want 1", code)
		}
	})
	if !strings.Contains(out, "removed only_a.txt") {
		t.Errorf("expected `removed only_a.txt` line, got %q", out)
	}
}

func TestCmdDiffTreeJSON(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if err := os.WriteFile(filepath.Join(b, "only_b.txt"), []byte("y"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := captureStdout(t, func() {
		_ = cmdDiffTree(context.Background(), config.Defaults(),
			[]string{"--quiet", "--json", a, b})
	})
	if !strings.Contains(out, `"status":"added"`) || !strings.Contains(out, `"path":"only_b.txt"`) {
		t.Errorf("--json output missing expected fields: %s", out)
	}
}
