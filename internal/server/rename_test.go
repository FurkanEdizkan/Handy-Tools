package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/rename"
)

func TestRenameHandlerInspectReturnsPlan(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &RenameHandler{Opts: Options{AllowRoots: []string{dir}}}
	ins, err := h.Inspect(RenameParams{
		Sources: []string{src},
		Pattern: `^old\.txt$`,
		Replace: "new.txt",
	})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(ins.Plans) != 1 || ins.Plans[0].To != filepath.Join(dir, "new.txt") {
		t.Fatalf("unexpected plan: %#v", ins.Plans)
	}
}

func TestRenameHandlerRunRenamesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &RenameHandler{Opts: Options{AllowRoots: []string{dir}}}
	err := h.Run(context.Background(), RenameParams{
		Sources: []string{src},
		Pattern: `^old\.txt$`,
		Replace: "new.txt",
	}, func(p tools.Progress) error { return nil })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Fatalf("renamed file not present: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("old file should be gone, got err=%v", err)
	}
}

func TestRenameHandlerRejectsPathOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	h := &RenameHandler{Opts: Options{AllowRoots: []string{dir}}}
	err := h.Run(context.Background(), RenameParams{
		Sources: []string{"/tmp/outside.txt"},
		Pattern: `.*`,
		Replace: "x",
	}, func(p tools.Progress) error { return nil })
	if err == nil {
		t.Fatal("expected path-outside-root error")
	}
}

// escapeFixture lays out <base>/root/victim.txt plus a sibling <base>/outside
// directory, returning the root, the victim path, and the escape target a
// `../outside/moved.txt` replacement would produce.
func escapeFixture(t *testing.T) (root, victim, escaped string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{root, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	victim = filepath.Join(root, "victim.txt")
	if err := os.WriteFile(victim, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, victim, filepath.Join(outside, "moved.txt")
}

// TestRenameHandlerRunRejectsTargetOutsideRoot: a replacement that climbs out
// of the source directory must not move the file outside the allow-roots.
func TestRenameHandlerRunRejectsTargetOutsideRoot(t *testing.T) {
	root, victim, escaped := escapeFixture(t)
	h := &RenameHandler{Opts: Options{AllowRoots: []string{root}}}
	var terminalErr *tools.Error
	err := h.Run(context.Background(), RenameParams{
		Sources: []string{victim},
		Pattern: `^victim\.txt$`,
		Replace: "../outside/moved.txt",
	}, func(p tools.Progress) error {
		if p.Completed {
			terminalErr = p.Err
		}
		return nil
	})
	if err == nil && terminalErr == nil {
		t.Fatal("expected the rename to be rejected")
	}
	if _, statErr := os.Stat(escaped); !os.IsNotExist(statErr) {
		t.Fatalf("file moved outside the allow-root (stat err=%v)", statErr)
	}
	if _, statErr := os.Stat(victim); statErr != nil {
		t.Fatalf("source should be untouched: %v", statErr)
	}
}

// TestRenameHandlerInspectRejectsTargetOutsideRoot: the dry-run must not
// present a plan whose target escapes the allow-roots.
func TestRenameHandlerInspectRejectsTargetOutsideRoot(t *testing.T) {
	root, victim, _ := escapeFixture(t)
	h := &RenameHandler{Opts: Options{AllowRoots: []string{root}}}
	if _, err := h.Inspect(RenameParams{
		Sources: []string{victim},
		Pattern: `^victim\.txt$`,
		Replace: "../outside/moved.txt",
	}); err == nil {
		t.Fatal("expected Inspect to reject a plan target outside the allow-root")
	}
}

// TestRenameHandlerCheckTargets exercises the transport-level target guard
// directly, independent of whatever the rename package itself validates.
func TestRenameHandlerCheckTargets(t *testing.T) {
	root := t.TempDir()
	h := &RenameHandler{Opts: Options{AllowRoots: []string{root}}}
	inside := rename.Plan{From: filepath.Join(root, "a"), To: filepath.Join(root, "b")}
	if err := h.checkTargets([]rename.Plan{inside}); err != nil {
		t.Fatalf("in-root target rejected: %v", err)
	}
	escape := rename.Plan{From: filepath.Join(root, "a"), To: "/etc/escaped"}
	err := h.checkTargets([]rename.Plan{inside, escape})
	var te *tools.Error
	if !errors.As(err, &te) || te.Code != tools.CodeBadRequest {
		t.Fatalf("checkTargets(escape) = %v, want a %s *tools.Error", err, tools.CodeBadRequest)
	}
}
