package image

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// writePreviewPNG writes a w×h PNG fixture for the preview tests.
func writePreviewPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

// TestPreviewDownscales: an 800×600 source fit into a 200×200 box yields a
// 200×150 PNG (scaled to fit, aspect preserved) — never upscaled.
func TestPreviewDownscales(t *testing.T) {
	src := filepath.Join(t.TempDir(), "big.png")
	writePreviewPNG(t, src, 800, 600)

	res, err := Preview(PreviewRequest{Source: src, MaxWidth: 200, MaxHeight: 200})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if res.Width != 200 || res.Height != 150 {
		t.Fatalf("got %dx%d, want 200x150", res.Width, res.Height)
	}
	if _, err := png.Decode(bytes.NewReader(res.PNG)); err != nil {
		t.Fatalf("returned bytes are not a valid PNG: %v", err)
	}
}

// TestPreviewDefaultBound: unset bounds fall back to DefaultPreviewBound.
func TestPreviewDefaultBound(t *testing.T) {
	src := filepath.Join(t.TempDir(), "square.png")
	writePreviewPNG(t, src, 1000, 1000)

	res, err := Preview(PreviewRequest{Source: src})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if res.Width != DefaultPreviewBound || res.Height != DefaultPreviewBound {
		t.Fatalf("got %dx%d, want %d square", res.Width, res.Height, DefaultPreviewBound)
	}
}

// TestPreviewSmallImageNotUpscaled: a source smaller than the box is returned
// at its own size.
func TestPreviewSmallImageNotUpscaled(t *testing.T) {
	src := filepath.Join(t.TempDir(), "small.png")
	writePreviewPNG(t, src, 40, 30)

	res, err := Preview(PreviewRequest{Source: src, MaxWidth: 320, MaxHeight: 320})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if res.Width != 40 || res.Height != 30 {
		t.Fatalf("got %dx%d, want 40x30 (no upscale)", res.Width, res.Height)
	}
}

func TestPreviewMissingFile(t *testing.T) {
	if _, err := Preview(PreviewRequest{Source: "/nonexistent-handy-xyz.png"}); err == nil {
		t.Fatal("expected an error for a missing source file")
	}
}
