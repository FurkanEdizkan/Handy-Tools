package rename

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// TestInspectRejectsReplacementLeavingDirectory: the replacement may only
// produce a new basename. A path separator, or a result of "", "." or "..",
// would move the file into another directory — possibly outside the server's
// allow-roots, which only ever see the source paths.
func TestInspectRejectsReplacementLeavingDirectory(t *testing.T) {
	cases := []struct {
		name, pattern, replace string
	}{
		{"parent traversal", `^a\.txt$`, "../../outside.txt"},
		{"subdirectory", `^a\.txt$`, "sub/a.txt"},
		{"empty result", `.*`, ""},
		{"dot", `^a\.txt$`, "."},
		{"dot-dot", `^a\.txt$`, ".."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			sub := filepath.Join(dir, "sub")
			if err := os.MkdirAll(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			src := seed(t, sub, "a.txt")
			req := Request{Sources: src, Pattern: c.pattern, Replace: c.replace, OnCollision: CollisionSuffix}

			if _, terr := Inspect(req); terr == nil || terr.Code != tools.CodeBadRequest {
				t.Fatalf("Inspect: want BAD_REQUEST, got %+v", terr)
			}
			last := drain(t, Run(context.Background(), req))
			if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
				t.Fatalf("Run: want BAD_REQUEST, got %+v", last.Err)
			}
			if _, err := os.Stat(src[0]); err != nil {
				t.Fatalf("source was moved: %v", err)
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 1 {
				t.Fatalf("something was created next to the source dir: %v", entries)
			}
		})
	}
}

// TestRunEveryMatchingRenameFailedIsAnError: rows that didn't match are not
// successes, so a batch whose only matching file failed must report an error
// (coalesced from the per-file codes) instead of exiting cleanly.
func TestRunEveryMatchingRenameFailedIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — file modes are bypassed")
	}
	dir := t.TempDir()
	srcs := seed(t, dir, "keep1.txt", "keep2.txt", "match.txt")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	last := drain(t, Run(context.Background(), Request{Sources: srcs, Pattern: `^match`, Replace: "renamed"}))
	if last.Err == nil || last.Err.Code != tools.CodePermissionDenied {
		t.Fatalf("want PERMISSION_DENIED terminal error, got %+v (msg %q)", last.Err, last.Message)
	}
	if len(last.Failures) != 1 {
		t.Fatalf("want 1 failure, got %+v", last.Failures)
	}
}

// TestRunCancelWithRollbackUndoesRenames: cancelling a --rollback-on-error
// batch must put back the renames already done, like a failure would.
func TestRunCancelWithRollbackUndoesRenames(t *testing.T) {
	dir := t.TempDir()
	names := make([]string, 50)
	for i := range names {
		names[i] = fmt.Sprintf("f%02d.txt", i)
	}
	srcs := seed(t, dir, names...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n := 0
	for range Run(ctx, Request{Sources: srcs, Pattern: `^f`, Replace: "g", Rollback: true}) {
		n++
		if n == 3 {
			cancel()
		}
	}

	renamed, _ := filepath.Glob(filepath.Join(dir, "g*.txt"))
	if len(renamed) != 0 {
		t.Fatalf("cancel left %d rename(s) in place: %v", len(renamed), renamed)
	}
	for _, p := range srcs {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s not restored: %v", filepath.Base(p), err)
		}
	}
}
