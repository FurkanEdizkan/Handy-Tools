package ui

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	jobqueue "github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// writeTestPNG drops a tiny 2×2 PNG at path for the image round-trip test.
func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode png: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close png: %v", err)
	}
}

// drainRunner runs a queue.Runner to completion and returns every progress
// event it emitted.
func drainRunner(t *testing.T, r jobqueue.Runner) []tools.Progress {
	t.Helper()
	var got []tools.Progress
	r(context.Background(), func(p tools.Progress) { got = append(got, p) })
	if len(got) == 0 {
		t.Fatal("runner emitted no progress")
	}
	return got
}

// TestRealRunnerConvertsImage drives the #153 runner end to end: a real PNG in,
// a real JPEG written to the chosen output directory, no error events.
func TestRealRunnerConvertsImage(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	writeTestPNG(t, src)
	out := filepath.Join(dir, "out")

	imgTool, ok := lookupTool("convert-image")
	if !ok {
		t.Fatal("convert-image tool not found")
	}
	job := RunJob{
		Tool:       imgTool,
		Files:      []fileItem{{Path: src, Name: "in.png", Target: "JPEG"}},
		Out:        outCustom,
		CustomPath: out,
		Quality:    80,
	}
	for _, p := range drainRunner(t, realRunner(job)) {
		if p.Err != nil {
			t.Fatalf("unexpected error event: %v", p.Err)
		}
	}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected an output file in %s; err=%v entries=%v", out, err, entries)
	}
}

// TestRealRunnerRejectsEmptySelection confirms a job with no real file paths
// (only demo sample rows) terminates with a structured error.
func TestRealRunnerRejectsEmptySelection(t *testing.T) {
	imgTool, ok := lookupTool("convert-image")
	if !ok {
		t.Fatal("convert-image tool not found")
	}
	job := RunJob{Tool: imgTool, Files: []fileItem{{Name: "demo.png"}}} // empty Path
	prog := drainRunner(t, realRunner(job))
	if last := prog[len(prog)-1]; last.Err == nil {
		t.Fatalf("expected an error for an empty selection, got %+v", last)
	}
}

// TestRealRunnerPacksArchive drives the archive-pack path: a real file in, a
// real zip written.
func TestRealRunnerPacksArchive(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	out := filepath.Join(dir, "out")

	packTool, ok := lookupTool("zip-pack")
	if !ok {
		t.Fatal("zip-pack tool not found")
	}
	job := RunJob{
		Tool:        packTool,
		ArchiveMode: archivePack,
		ArchiveOut:  "zip",
		Files:       []fileItem{{Path: src, Name: "a.txt"}},
		Out:         outCustom,
		CustomPath:  out,
	}
	for _, p := range drainRunner(t, realRunner(job)) {
		if p.Err != nil {
			t.Fatalf("unexpected pack error: %v", p.Err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "archive.zip")); err != nil {
		t.Fatalf("expected archive.zip in %s: %v", out, err)
	}
}
