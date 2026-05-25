package difftree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestInspectAddedRemovedChanged(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	write(t, filepath.Join(a, "keep.txt"), "same")
	write(t, filepath.Join(b, "keep.txt"), "same")
	write(t, filepath.Join(a, "only_a.txt"), "x")
	write(t, filepath.Join(b, "only_b.txt"), "y")
	write(t, filepath.Join(a, "changed.txt"), "old")
	write(t, filepath.Join(b, "changed.txt"), "newer-and-longer")

	// Force equal mtime on keep so the mtime-mode comparison doesn't flag it.
	now := time.Now()
	for _, p := range []string{filepath.Join(a, "keep.txt"), filepath.Join(b, "keep.txt")} {
		if err := os.Chtimes(p, now, now); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}

	diffs, terr := Inspect(Request{A: a, B: b})
	if terr != nil {
		t.Fatalf("inspect: %v", terr)
	}
	got := map[string]Diff{}
	for _, d := range diffs {
		got[d.Path] = d
	}
	if d := got["only_a.txt"]; d.Status != StatusRemoved {
		t.Errorf("only_a.txt status = %q, want removed", d.Status)
	}
	if d := got["only_b.txt"]; d.Status != StatusAdded {
		t.Errorf("only_b.txt status = %q, want added", d.Status)
	}
	if d := got["changed.txt"]; d.Status != StatusChanged || d.Reason != "size" {
		t.Errorf("changed.txt = %+v, want changed/size", d)
	}
	if _, ok := got["keep.txt"]; ok {
		t.Errorf("keep.txt should not be reported")
	}
}

func TestInspectHashCatchesContentChangeWithSameMtimeAndSize(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	// Same length, different content — invisible to size+mtime mode after
	// equalising mtimes, but caught by SHA256.
	write(t, filepath.Join(a, "f.txt"), "AAAA")
	write(t, filepath.Join(b, "f.txt"), "BBBB")
	stamp := time.Unix(1_700_000_000, 0)
	for _, p := range []string{filepath.Join(a, "f.txt"), filepath.Join(b, "f.txt")} {
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}

	mtimeDiffs, terr := Inspect(Request{A: a, B: b, Mode: ModeMTime})
	if terr != nil {
		t.Fatalf("inspect mtime: %v", terr)
	}
	if len(mtimeDiffs) != 0 {
		t.Errorf("mtime mode shouldn't notice content-only change, got %+v", mtimeDiffs)
	}

	hashDiffs, terr := Inspect(Request{A: a, B: b, Mode: ModeHash})
	if terr != nil {
		t.Fatalf("inspect hash: %v", terr)
	}
	if len(hashDiffs) != 1 || hashDiffs[0].Status != StatusChanged || hashDiffs[0].Reason != "hash" {
		t.Errorf("hash mode should report changed/hash, got %+v", hashDiffs)
	}
}

func TestInspectSymlinkLoopDoesNotHang(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	// Build a symlink loop inside A: a/sub/loop -> ../sub
	sub := filepath.Join(a, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.Symlink("../sub", filepath.Join(sub, "loop")); err != nil {
		t.Skipf("symlink not supported on this filesystem: %v", err)
	}
	write(t, filepath.Join(sub, "real.txt"), "hello")

	// Bound the call so a regression that started following symlinks would
	// trip the test instead of hanging forever.
	done := make(chan struct{})
	var diffs []Diff
	var terr *tools.Error
	go func() {
		defer close(done)
		diffs, terr = Inspect(Request{A: a, B: b})
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Inspect hung on symlink loop")
	}
	if terr != nil {
		t.Fatalf("inspect: %v", terr)
	}
	// real.txt should appear as removed; the loop symlink itself should be
	// skipped (we never follow symlinks).
	wantRemoved := false
	for _, d := range diffs {
		if d.Status == StatusRemoved && d.Path == "sub/real.txt" {
			wantRemoved = true
		}
		if d.Path == "sub/loop" {
			t.Errorf("loop symlink should not appear as a diff entry, got %+v", d)
		}
	}
	if !wantRemoved {
		t.Errorf("sub/real.txt should be reported removed (only in A), got diffs: %+v", diffs)
	}
}

func TestInspectRejectsBadRequest(t *testing.T) {
	if _, terr := Inspect(Request{}); terr == nil || terr.Code != tools.CodeBadRequest {
		t.Errorf("empty paths should be BAD_REQUEST, got %+v", terr)
	}
	// Non-directory root.
	tmp := t.TempDir()
	f := filepath.Join(tmp, "f")
	write(t, f, "x")
	if _, terr := Inspect(Request{A: f, B: tmp}); terr == nil || terr.Code != tools.CodeBadRequest {
		t.Errorf("file-as-root should be BAD_REQUEST, got %+v", terr)
	}
	if _, terr := Inspect(Request{A: tmp, B: tmp, Mode: "mystery"}); terr == nil || terr.Code != tools.CodeBadRequest {
		t.Errorf("unknown mode should be BAD_REQUEST, got %+v", terr)
	}
}

func TestRunEmitsTerminalSummary(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	write(t, filepath.Join(a, "only_a.txt"), "x")
	write(t, filepath.Join(b, "only_b.txt"), "y")

	ch := Run(context.Background(), Request{A: a, B: b})
	var last tools.Progress
	for ev := range ch {
		last = ev
	}
	if !last.Completed || last.Err != nil {
		t.Fatalf("expected success, got %+v", last)
	}
	// Summary message carries the counts.
	want := "1 added, 1 removed, 0 changed"
	if !strings.Contains(last.Message, want) {
		t.Errorf("summary = %q, want substring %q", last.Message, want)
	}
}

func TestParseMode(t *testing.T) {
	cases := []struct {
		in   string
		want Mode
		ok   bool
	}{
		{"", ModeMTime, true},
		{"mtime", ModeMTime, true},
		{"MTIME", ModeMTime, true},
		{"hash", ModeHash, true},
		{"galaxy", "", false},
	}
	for _, tc := range cases {
		got, ok := ParseMode(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseMode(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
