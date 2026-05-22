package server

import (
	"context"
	stdimage "image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	handytoolsv1 "github.com/furkandedizkan/handy-tools/gen/handytools/v1"
)

// writeGRPCTestPNG writes a w×h PNG fixture for the gRPC preview tests.
func writeGRPCTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 220, G: 90, B: 40, A: 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

// TestGRPCImagePreview drives the unary ImageService.Preview handler: it
// downscales within the requested bounds and returns inline PNG bytes.
func TestGRPCImagePreview(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	writeGRPCTestPNG(t, src, 600, 400)

	g := &grpcImageServer{h: &ImageHandler{Opts: Options{AllowRoots: []string{dir}}}}
	resp, err := g.Preview(context.Background(), &handytoolsv1.ImagePreviewRequest{
		Source:    &handytoolsv1.FileRef{Path: src},
		MaxWidth:  150,
		MaxHeight: 150,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if resp.GetWidth() > 150 || resp.GetHeight() > 150 || resp.GetWidth() == 0 {
		t.Fatalf("thumbnail not downscaled into 150×150: %d×%d", resp.GetWidth(), resp.GetHeight())
	}
	if len(resp.GetImage()) == 0 {
		t.Fatal("expected inline PNG bytes")
	}
	if resp.GetFormat() != handytoolsv1.ImageFormat_IMAGE_FORMAT_PNG {
		t.Fatalf("format = %v, want PNG", resp.GetFormat())
	}
}

// TestGRPCImagePreviewRejectsOutsideRoot confirms the Preview handler runs the
// source through the same allow-root sandbox as the other RPCs.
func TestGRPCImagePreviewRejectsOutsideRoot(t *testing.T) {
	g := &grpcImageServer{h: &ImageHandler{Opts: Options{AllowRoots: []string{t.TempDir()}}}}
	if _, err := g.Preview(context.Background(), &handytoolsv1.ImagePreviewRequest{
		Source: &handytoolsv1.FileRef{Path: "/etc/hostname"},
	}); err == nil {
		t.Fatal("expected a CheckPath rejection for a path outside the allow-root")
	}
}
