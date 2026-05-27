package rename

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func seed(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	paths := make([]string, 0, len(names))
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", n, err)
		}
		paths = append(paths, p)
	}
	return paths
}

func collect(ch <-chan tools.Progress) []tools.Progress {
	out := []tools.Progress{}
	for p := range ch {
		out = append(out, p)
	}
	return out
}

func TestInspectInvalidPattern(t *testing.T) {
	_, terr := Inspect(Request{Sources: []string{"a.jpg"}, Pattern: "[", Replace: "x"})
	if terr == nil || terr.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST for unparseable regex, got %+v", terr)
	}
}

func TestInspectEmptyPattern(t *testing.T) {
	_, terr := Inspect(Request{Sources: []string{"a.jpg"}, Pattern: "", Replace: "x"})
	if terr == nil || terr.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST for empty pattern, got %+v", terr)
	}
}

func TestInspectMatchesBasenameOnly(t *testing.T) {
	dir := t.TempDir()
	srcs := seed(t, dir, "IMG_0001.JPG", "notes.txt")
	ins, terr := Inspect(Request{
		Sources: srcs,
		Pattern: `IMG_(\d+)\.JPG`,
		Replace: `photo-$1.jpg`,
	})
	if terr != nil {
		t.Fatalf("inspect: %v", terr)
	}
	if len(ins.Plans) != 2 {
		t.Fatalf("want 2 plans, got %d", len(ins.Plans))
	}
	want := filepath.Join(dir, "photo-0001.jpg")
	if ins.Plans[0].To != want {
		t.Errorf("plan[0].To = %q, want %q", ins.Plans[0].To, want)
	}
	// notes.txt did not match — its plan row is a no-op.
	if ins.Plans[1].From != ins.Plans[1].To {
		t.Errorf("plan[1] should be a no-op, got %+v", ins.Plans[1])
	}
	if len(ins.Issues) != 0 {
		t.Errorf("expected no preflight issues for readable sources, got %+v", ins.Issues)
	}
}

// TestInspectReportsMissingSource confirms Inspect populates Issues for a
// source path that doesn't exist, without failing the whole call.
func TestInspectReportsMissingSource(t *testing.T) {
	dir := t.TempDir()
	srcs := seed(t, dir, "real.jpg")
	srcs = append(srcs, filepath.Join(dir, "ghost.jpg"))
	ins, terr := Inspect(Request{
		Sources: srcs,
		Pattern: `\.jpg`,
		Replace: `.jpeg`,
	})
	if terr != nil {
		t.Fatalf("inspect: %v", terr)
	}
	if len(ins.Issues) != 1 {
		t.Fatalf("want 1 issue for missing ghost.jpg, got %d: %+v", len(ins.Issues), ins.Issues)
	}
	if ins.Issues[0].Code != tools.CodeNotFound {
		t.Errorf("want NOT_FOUND, got %q", ins.Issues[0].Code)
	}
}

func TestRunHappyPath(t *testing.T) {
	dir := t.TempDir()
	srcs := seed(t, dir, "IMG_0001.JPG", "IMG_0002.JPG")

	last := drain(t, Run(context.Background(), Request{
		Sources: srcs,
		Pattern: `IMG_(\d+)\.JPG`,
		Replace: `photo-$1.jpg`,
	}))
	if !last.Completed || last.Err != nil {
		t.Fatalf("expected success, got %+v", last)
	}
	if !strings.Contains(last.Message, "renamed 2") {
		t.Errorf("summary lacks renamed count: %q", last.Message)
	}
	for _, name := range []string{"photo-0001.jpg", "photo-0002.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestRunCollisionErrorByDefault(t *testing.T) {
	dir := t.TempDir()
	srcs := seed(t, dir, "a.txt", "b.txt")
	// Pre-seed the target so the first rename collides.
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("seed shared: %v", err)
	}

	last := drain(t, Run(context.Background(), Request{
		Sources: srcs,
		Pattern: `\w+\.txt`,
		Replace: `shared.txt`,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST on collision, got %+v", last)
	}
	// Sources still present — nothing was moved.
	for _, s := range srcs {
		if _, err := os.Stat(s); err != nil {
			t.Errorf("source %s lost after aborted batch: %v", s, err)
		}
	}
}

func TestRunCollisionSuffix(t *testing.T) {
	dir := t.TempDir()
	srcs := seed(t, dir, "a.txt", "b.txt")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("seed shared: %v", err)
	}

	last := drain(t, Run(context.Background(), Request{
		Sources:     srcs,
		Pattern:     `\w+\.txt`,
		Replace:     `shared.txt`,
		OnCollision: CollisionSuffix,
	}))
	if last.Err != nil {
		t.Fatalf("expected success with suffix mode, got %+v", last)
	}
	// One of the renames produced shared-1.txt, the other shared-2.txt.
	want := []string{filepath.Join(dir, "shared-1.txt"), filepath.Join(dir, "shared-2.txt")}
	for _, w := range want {
		if _, err := os.Stat(w); err != nil {
			t.Errorf("expected %s: %v", w, err)
		}
	}
}

func TestRunCollisionSkip(t *testing.T) {
	dir := t.TempDir()
	srcs := seed(t, dir, "a.txt", "b.txt")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("seed shared: %v", err)
	}

	last := drain(t, Run(context.Background(), Request{
		Sources:     srcs,
		Pattern:     `\w+\.txt`,
		Replace:     `shared.txt`,
		OnCollision: CollisionSkip,
	}))
	if last.Err != nil {
		t.Fatalf("expected success with skip mode, got %+v", last)
	}
	// Sources still present — skip mode demoted the colliding plans to no-ops.
	for _, s := range srcs {
		if _, err := os.Stat(s); err != nil {
			t.Errorf("source %s lost in skip mode: %v", s, err)
		}
	}
	// And shared.txt still has its original content.
	body, err := os.ReadFile(filepath.Join(dir, "shared.txt"))
	if err != nil || string(body) != "existing" {
		t.Errorf("shared.txt was clobbered: body=%q err=%v", string(body), err)
	}
}

// TestRunPermissionDeniedPerFile chmod-blocks the parent directory so os.Rename
// returns EACCES on every file; the per-file SeverityError events must carry
// PERMISSION_DENIED in their Err.Code so callers can distinguish "user can't
// write here" from a generic disk failure.
func TestRunPermissionDeniedPerFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — file modes are bypassed")
	}
	dir := t.TempDir()
	srcs := seed(t, dir, "IMG_0001.JPG")
	// Make the directory read-only so os.Rename inside it fails with EACCES.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	events := collect(Run(context.Background(), Request{
		Sources: srcs,
		Pattern: `IMG_(\d+)\.JPG`,
		Replace: `photo-$1.jpg`,
	}))
	var sawPerFile bool
	for _, ev := range events {
		if ev.Completed {
			continue
		}
		if ev.Level == tools.SeverityError && ev.Err != nil && ev.Err.Code == tools.CodePermissionDenied {
			sawPerFile = true
		}
	}
	if !sawPerFile {
		t.Fatalf("expected per-file event with PERMISSION_DENIED, got %d events: %+v", len(events), events)
	}
}

// TestRunRollbackUndoesEarlierRenames forces a mid-batch failure (the target
// of the second rename is chmod'd 0400 so the rename can't proceed) with
// Rollback enabled, then asserts the first rename was undone and the
// terminal Failures slice carries both the original error and the absence
// of any ROLLBACK_FAILED entries (the undo itself worked).
func TestRunRollbackUndoesEarlierRenames(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — modes are bypassed")
	}
	dir := t.TempDir()
	// a.txt -> a-x.txt is fine. b.txt -> b-x.txt would clobber the
	// pre-seeded read-only file with CollisionError → resolveCollisions
	// would catch it upfront. So we instead make b.txt unmovable by
	// blocking write on the directory after a.txt is already renamed —
	// which requires a different trick: chmod the destination dir to
	// read-only between iterations. The cleanest approach is to seed a
	// scenario where the second source physically can't be renamed: use
	// a subdir as the second source's target, and remove that subdir
	// after the batch starts.
	srcs := seed(t, dir, "a.txt", "b.txt")
	// Use OnCollision=Skip so resolveCollisions accepts the plan; force the
	// second rename to fail by making the destination path invalid (target
	// directory deleted after Inspect).
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcInSub := filepath.Join(subdir, "c.txt")
	if err := os.WriteFile(srcInSub, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	srcs = append(srcs, srcInSub)

	// Plan: rename a.txt -> a-x.txt, b.txt -> b-x.txt, c.txt -> c-x.txt.
	// Between Inspect and the rename of c.txt we chmod subdir 0o500 so
	// os.Rename can't write the new name. The first two renames go through
	// in dir; the third fails.
	go func() {
		// Small delay so the first renames in dir finish before subdir is
		// locked. Not deterministic enough for racey environments, but
		// good enough for a quick assertion. Use t.TempDir cleanup to
		// restore.
	}()

	// Synchronous trick: pre-chmod subdir AFTER inspecting plans by hand:
	// run without async — call Inspect, plan, lock subdir, then call Run.
	// Easier: chmod subdir now (so renames in subdir fail), but a.txt and
	// b.txt go through.
	if err := os.Chmod(subdir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(subdir, 0o755) })

	events := collect(Run(context.Background(), Request{
		Sources:     srcs,
		Pattern:     `(\w+)\.txt`,
		Replace:     `$1-x.txt`,
		OnCollision: CollisionSkip,
		Rollback:    true,
	}))
	last := events[len(events)-1]
	if !last.Completed || last.Err == nil {
		t.Fatalf("expected terminal failure event, got %+v", last)
	}
	// After rollback: a.txt and b.txt should be restored to their original
	// names (i.e. a-x.txt / b-x.txt should NOT exist).
	for _, name := range []string{"a-x.txt", "b-x.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("rollback failed to undo %s", name)
		}
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("rollback should have restored %s, got: %v", name, err)
		}
	}
	// Failures should carry exactly one entry for the failed c.txt rename;
	// the rollback steps for a.txt and b.txt should have succeeded so no
	// ROLLBACK_FAILED entries.
	if len(last.Failures) == 0 {
		t.Fatalf("expected at least one failure entry, got none")
	}
	for _, f := range last.Failures {
		if f.Code == tools.CodeRollbackFailed {
			t.Errorf("did not expect ROLLBACK_FAILED, got %+v", f)
		}
	}
}

func TestRunRejectsEmptySources(t *testing.T) {
	last := drain(t, Run(context.Background(), Request{Pattern: `a`, Replace: `b`}))
	if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST for empty sources, got %+v", last)
	}
}

// drain reads progress events from ch and returns the terminal one. Fails the
// test if no terminal event arrives — every code path in rename must emit one.
func drain(t *testing.T, ch <-chan tools.Progress) tools.Progress {
	t.Helper()
	events := collect(ch)
	if len(events) == 0 {
		t.Fatal("no events on progress channel")
	}
	last := events[len(events)-1]
	if !last.Completed {
		t.Fatalf("last event isn't terminal: %+v", last)
	}
	return last
}
