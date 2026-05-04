package image

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy/internal/tools"
)

func writeTinyPNG(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "in.png")
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 64), G: uint8(y * 64), B: 128, A: 255})
		}
	}
	f, err := os.Create(src)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return src
}

func collect(ch <-chan tools.Progress) []tools.Progress {
	out := []tools.Progress{}
	for p := range ch {
		out = append(out, p)
	}
	return out
}

func TestConvertPNGToJPEG(t *testing.T) {
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "out.jpg")

	progress := collect(Convert(context.Background(), ConvertRequest{
		Source:       src,
		TargetFormat: FormatJPEG,
		Output:       dst,
		Opts:         Options{Quality: 80},
	}))

	last := progress[len(progress)-1]
	if !last.Completed || last.Err != nil {
		t.Fatalf("expected success, got %+v", last)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("output missing: %v", err)
	}
}

func TestConvertResizeShrinks(t *testing.T) {
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "out.png")

	progress := collect(Convert(context.Background(), ConvertRequest{
		Source:       src,
		TargetFormat: FormatPNG,
		Output:       dst,
		Opts:         Options{MaxWidth: 2},
	}))

	if last := progress[len(progress)-1]; last.Err != nil {
		t.Fatalf("convert: %v", last.Err)
	}
	f, err := os.Open(dst)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if w := img.Bounds().Dx(); w != 2 {
		t.Fatalf("expected width 2, got %d", w)
	}
}

func TestConvertRefusesOverwriteByDefault(t *testing.T) {
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "out.png")
	if err := os.WriteFile(dst, []byte("preexisting"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	progress := collect(Convert(context.Background(), ConvertRequest{
		Source:       src,
		TargetFormat: FormatPNG,
		Output:       dst,
	}))
	last := progress[len(progress)-1]
	if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST, got %+v", last)
	}
}

func TestWebPEncodeNotImplemented(t *testing.T) {
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "out.webp")

	progress := collect(Convert(context.Background(), ConvertRequest{
		Source:       src,
		TargetFormat: FormatWebP,
		Output:       dst,
	}))
	if last := progress[len(progress)-1]; last.Err == nil {
		t.Fatalf("expected error for webp encode")
	}
}
