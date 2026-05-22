package upload

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(filepath.Join(t.TempDir(), "uploads"), 1<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestCreateMakesIsolatedTree(t *testing.T) {
	m := newTestManager(t)
	a, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	b, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID == b.ID {
		t.Fatalf("two workspaces share an ID: %q", a.ID)
	}
	for _, d := range []string{a.InDir, a.OutDir} {
		info, err := os.Stat(d)
		if err != nil {
			t.Fatalf("stat %s: %v", d, err)
		}
		if perm := info.Mode().Perm(); perm != dirPerm {
			t.Errorf("dir %s perm = %o, want %o", d, perm, dirPerm)
		}
	}
}

func TestSaveWritesContent(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sf, err := m.Save(ws, "photo.png", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if sf.Name != "photo.png" {
		t.Errorf("Name = %q, want photo.png", sf.Name)
	}
	got, err := os.ReadFile(sf.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q, want hello", got)
	}
	if len(ws.Files) != 1 {
		t.Errorf("ws.Files = %d, want 1", len(ws.Files))
	}
}

func TestSaveDeduplicatesNames(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	a, err := m.Save(ws, "photo.png", strings.NewReader("FIRST"))
	if err != nil {
		t.Fatalf("Save A: %v", err)
	}
	b, err := m.Save(ws, "photo.png", strings.NewReader("SECOND"))
	if err != nil {
		t.Fatalf("Save B: %v", err)
	}
	if a.Path == b.Path {
		t.Fatalf("two same-named uploads collided on %q", a.Path)
	}
	if b.Name != "photo-1.png" {
		t.Errorf("second name = %q, want photo-1.png", b.Name)
	}
	for path, want := range map[string]string{a.Path: "FIRST", b.Path: "SECOND"} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("content at %s = %q, want %q", path, got, want)
		}
	}
}

func TestSaveSanitizesTraversal(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, name := range []string{"../../etc/passwd", "../escape.png", `..\..\win.png`, "/abs/path.png"} {
		sf, err := m.Save(ws, name, strings.NewReader("x"))
		if err != nil {
			t.Fatalf("Save(%q): %v", name, err)
		}
		if dir := filepath.Dir(sf.Path); dir != ws.InDir {
			t.Errorf("Save(%q) landed in %q, want inside %q", name, dir, ws.InDir)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"photo.png":        "photo.png",
		"../../etc/passwd": "passwd",
		`..\..\win.png`:    "win.png",
		"/abs/path.png":    "path.png",
		"":                 "upload",
		"..":               "upload",
		".":                "upload",
		"  spaced.png  ":   "spaced.png",
	}
	for in, want := range cases {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemoveIsIdempotent(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.Remove(ws.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(ws.Root); !os.IsNotExist(err) {
		t.Errorf("workspace root still present after Remove")
	}
	if _, ok := m.Get(ws.ID); ok {
		t.Errorf("workspace still registered after Remove")
	}
	if err := m.Remove(ws.ID); err != nil {
		t.Errorf("second Remove: %v", err)
	}
	if err := m.Remove("never-existed"); err != nil {
		t.Errorf("Remove(unknown): %v", err)
	}
}

func TestReapDeletesStale(t *testing.T) {
	m := newTestManager(t)
	m.TTL = time.Hour

	clock := time.Now()
	m.now = func() time.Time { return clock }

	stale, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	clock = clock.Add(2 * time.Hour)
	fresh, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	m.Reap()

	if _, err := os.Stat(stale.Root); !os.IsNotExist(err) {
		t.Errorf("stale workspace survived Reap")
	}
	if _, ok := m.Get(stale.ID); ok {
		t.Errorf("stale workspace still registered after Reap")
	}
	if _, err := os.Stat(fresh.Root); err != nil {
		t.Errorf("fresh workspace was reaped: %v", err)
	}
}

func TestStartReaper(t *testing.T) {
	m := newTestManager(t)
	m.TTL = 20 * time.Millisecond

	ws, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.StartReaper(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ws.Root); os.IsNotExist(err) {
			return // reaped
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("reaper did not delete the stale workspace in time")
}
