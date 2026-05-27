package stripmeta

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func seedJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
}

func collect(ch <-chan tools.Progress) []tools.Progress {
	out := []tools.Progress{}
	for p := range ch {
		out = append(out, p)
	}
	return out
}

func terminal(t *testing.T, events []tools.Progress) tools.Progress {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("no events on progress channel")
	}
	last := events[len(events)-1]
	if !last.Completed {
		t.Fatalf("last event not terminal: %+v", last)
	}
	return last
}

func TestRunHappyPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.jpg")
	seedJPEG(t, src)

	last := terminal(t, collect(Run(context.Background(), Request{Sources: []string{src}})))
	if last.Err != nil {
		t.Fatalf("expected success, got %+v", last)
	}
	out := filepath.Join(dir, "photo-stripped.jpg")
	if _, err := os.Stat(out); err != nil {
		t.Errorf("expected %s to exist: %v", out, err)
	}
}

func TestRunInPlace(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.jpg")
	seedJPEG(t, src)

	last := terminal(t, collect(Run(context.Background(), Request{
		Sources: []string{src}, InPlace: true,
	})))
	if last.Err != nil {
		t.Fatalf("expected success, got %+v", last)
	}
	// -stripped sibling should NOT exist (in-place mode overwrites source).
	if _, err := os.Stat(filepath.Join(dir, "photo-stripped.jpg")); err == nil {
		t.Errorf("--in-place should not create a -stripped sibling")
	}
	// No .handy-bak should be left over when Rollback is off.
	if _, err := os.Stat(src + ".handy-bak"); !os.IsNotExist(err) {
		t.Errorf("expected no .handy-bak without Rollback, got err = %v", err)
	}
}

func TestRunUnknownExtensionFailsClassified(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(p, []byte("plain text"), 0o644); err != nil {
		t.Fatal(err)
	}
	last := terminal(t, collect(Run(context.Background(), Request{Sources: []string{p}})))
	if last.Err == nil || last.Err.Code != tools.CodeUnsupportedInput {
		t.Fatalf("expected terminal UNSUPPORTED_INPUT (coalesced from per-file), got %+v", last.Err)
	}
	if len(last.Failures) != 1 {
		t.Errorf("want 1 failure entry, got %d: %+v", len(last.Failures), last.Failures)
	}
}

func TestRunInPlaceRollbackRestoresOriginal(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "photo.jpg")
	seedJPEG(t, good)
	originalBytes, _ := os.ReadFile(good)

	// Mix one valid image with one unknown extension. With Rollback the
	// batch should stop on the unknown ext and restore the JPEG from its
	// .handy-bak snapshot.
	bad := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(bad, []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}

	last := terminal(t, collect(Run(context.Background(), Request{
		Sources: []string{good, bad}, InPlace: true, Rollback: true,
	})))
	if last.Err == nil {
		t.Fatalf("expected terminal failure (rollback aborted), got %+v", last)
	}
	// The JPEG should be byte-identical to what it was before.
	now, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(now, originalBytes) {
		t.Errorf("rollback did not restore the JPEG; bytes diverged")
	}
	// No .handy-bak left over after the rollback.
	if _, err := os.Stat(good + ".handy-bak"); !os.IsNotExist(err) {
		t.Errorf("rollback should remove .handy-bak, got err = %v", err)
	}
}

// TestRunMixedBatchScenario is the canonical multi-file failure scenario for
// strip-meta: one normal JPEG, one chmod-blocked JPEG, and one unsupported
// extension. The batch continues past the per-file failures; the terminal
// event has Err == nil (partial success) and Failures lists exactly the
// two failed paths with the right codes. Mirrors the same scenario tests
// in internal/tools/{hash,rename,image,archive}.
func TestRunMixedBatchScenario(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — file modes are bypassed")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.jpg")
	seedJPEG(t, good)
	blocked := filepath.Join(dir, "blocked.jpg")
	seedJPEG(t, blocked)
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o644) })
	unsupported := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(unsupported, []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}

	last := terminal(t, collect(Run(context.Background(), Request{
		Sources: []string{good, blocked, unsupported},
	})))
	if last.Err != nil {
		t.Fatalf("expected partial-success terminal, got %+v", last.Err)
	}
	if len(last.Failures) != 2 {
		t.Fatalf("want 2 failures, got %d: %+v", len(last.Failures), last.Failures)
	}
	codes := map[string]string{}
	for _, f := range last.Failures {
		codes[f.Path] = f.Code
	}
	if codes[blocked] != tools.CodePermissionDenied {
		t.Errorf("blocked: want PERMISSION_DENIED, got %q", codes[blocked])
	}
	if codes[unsupported] != tools.CodeUnsupportedInput {
		t.Errorf("unsupported: want UNSUPPORTED_INPUT, got %q", codes[unsupported])
	}
	if _, err := os.Stat(filepath.Join(dir, "good-stripped.jpg")); err != nil {
		t.Errorf("expected good-stripped.jpg to exist alongside the failed siblings, got %v", err)
	}
}

func TestRunNonInPlaceRollbackDeletesWrittenOutputs(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "photo.jpg")
	seedJPEG(t, good)
	bad := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(bad, []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}

	last := terminal(t, collect(Run(context.Background(), Request{
		Sources: []string{good, bad}, Rollback: true,
	})))
	if last.Err == nil {
		t.Fatalf("expected terminal failure, got %+v", last)
	}
	// The -stripped output from the first (successful) file should be
	// removed by the rollback step.
	if _, err := os.Stat(filepath.Join(dir, "photo-stripped.jpg")); !os.IsNotExist(err) {
		t.Errorf("rollback should delete photo-stripped.jpg, got err = %v", err)
	}
}
