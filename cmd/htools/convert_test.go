package main

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/config"
)

// writeTinyPNG mirrors the 4×4 PNG helper used by the image package's own
// tests, so this integration test exercises the full convert → encode path
// without depending on real-world fixtures.
func writeTinyPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 64), G: uint8(y * 64), B: 128, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

// TestConvertEndToEnd runs the convert subcommand against a real 4×4 PNG
// and asserts a JPEG appears at the requested output path. This is the
// "happy-path smoke test" the plan called for.
func TestConvertEndToEnd(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	dst := filepath.Join(dir, "out.jpg")
	writeTinyPNG(t, src)

	cfg := config.Defaults()
	code := dispatch(context.Background(),
		cfg, "convert",
		[]string{"--format", "jpeg", "--quality", "70", "--out", dst, "--quiet", src},
	)
	if code != 0 {
		t.Fatalf("convert exit = %d, want 0", code)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("expected output at %s: %v", dst, err)
	}
}

// TestConvertMissingSourceFailsCleanly asserts the CLI surfaces a non-zero
// exit code (and doesn't panic) when the source path doesn't exist.
func TestConvertMissingSourceFailsCleanly(t *testing.T) {
	cfg := config.Defaults()
	code := dispatch(context.Background(),
		cfg, "convert",
		[]string{"--format", "jpeg", "--out", "/dev/null", "--quiet", "/nope/missing.png"},
	)
	if code == 0 {
		t.Errorf("expected non-zero exit for missing source, got 0")
	}
}
