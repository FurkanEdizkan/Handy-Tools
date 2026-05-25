package server

import (
	"path/filepath"
	"testing"
)

func TestCheckPathRejectsEmpty(t *testing.T) {
	if _, err := (Options{}).CheckPath(""); err == nil {
		t.Fatal("expected error on empty path")
	}
}

func TestCheckPathFailsClosedWithoutRoots(t *testing.T) {
	if _, err := (Options{}).CheckPath("/tmp/file"); err == nil {
		t.Fatal("expected error when no roots configured")
	}
}

func TestCheckPathAllowsUnderRoot(t *testing.T) {
	root := t.TempDir()
	abs, err := Options{AllowRoots: []string{root}}.CheckPath(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want, _ := filepath.Abs(filepath.Join(root, "a.txt"))
	if abs != want {
		t.Fatalf("path: got %q want %q", abs, want)
	}
}

func TestCheckPathRejectsTraversalOutsideRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := (Options{AllowRoots: []string{root}}).CheckPath("/etc/passwd"); err == nil {
		t.Fatal("expected error for path outside root")
	}
}

// TestCheckPathRejectsPrefixOnlyMatch documents that an allow-root of /foo
// must NOT accept paths under /foobar — the implementation guards against
// this by appending a separator to both sides before HasPrefix. A regression
// here would be a serious sandbox escape.
func TestCheckPathRejectsPrefixOnlyMatch(t *testing.T) {
	root := t.TempDir()
	sibling := root + "-sibling"
	if _, err := (Options{AllowRoots: []string{root}}).CheckPath(filepath.Join(sibling, "x.txt")); err == nil {
		t.Fatalf("expected rejection: %q must not match root %q", sibling, root)
	}
}

// TestCheckPathRejectsDotDotTraversal verifies filepath.Clean strips a `..`
// segment before the prefix check, so traversal can't escape the root.
func TestCheckPathRejectsDotDotTraversal(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "..", "etc", "passwd")
	if _, err := (Options{AllowRoots: []string{root}}).CheckPath(bad); err == nil {
		t.Fatalf("expected rejection of traversal path %q", bad)
	}
}

func TestCheckPathAllowsExactRoot(t *testing.T) {
	root := t.TempDir()
	abs, err := Options{AllowRoots: []string{root}}.CheckPath(root)
	if err != nil {
		t.Fatalf("exact-root path must be allowed: %v", err)
	}
	want, _ := filepath.Abs(root)
	if abs != want {
		t.Fatalf("path: got %q want %q", abs, want)
	}
}

func TestCheckPathMultipleRoots(t *testing.T) {
	r1 := t.TempDir()
	r2 := t.TempDir()
	opts := Options{AllowRoots: []string{r1, r2}}
	if _, err := opts.CheckPath(filepath.Join(r1, "a.txt")); err != nil {
		t.Fatalf("first root rejected: %v", err)
	}
	if _, err := opts.CheckPath(filepath.Join(r2, "b.txt")); err != nil {
		t.Fatalf("second root rejected: %v", err)
	}
	if _, err := opts.CheckPath("/tmp/c.txt"); err == nil {
		t.Fatal("path outside both roots must be rejected")
	}
}

// TestCheckPathRootAllowAcceptsAnyAbsolute locks in the htools-gui /
// htools-mcp default sandbox: AllowRoots:["/"] must allow any absolute path
// under root. A prior implementation double-appended the path separator and
// silently rejected everything under "/".
func TestCheckPathRootAllowAcceptsAnyAbsolute(t *testing.T) {
	opts := Options{AllowRoots: []string{"/"}}
	for _, p := range []string{"/tmp/x.txt", "/etc/hostname", "/"} {
		if _, err := opts.CheckPath(p); err != nil {
			t.Fatalf("CheckPath(%q): unexpected error %v", p, err)
		}
	}
}

func TestErrorFromToolNil(t *testing.T) {
	c, m, d := errorFromTool(nil)
	if c != "" || m != "" || d != "" {
		t.Fatalf("nil tool error must produce empty triple, got %q %q %q", c, m, d)
	}
}
