package image

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
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

// TestWebPEncodeViaMagick verifies WebP encoding is delegated to ImageMagick
// (#18): when magick is on PATH the conversion succeeds and writes the file;
// when it is absent the job fails with a MISSING_BINARY error rather than
// silently producing nothing.
func TestWebPEncodeViaMagick(t *testing.T) {
	sysdep.Reset()
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "out.webp")

	progress := collect(Convert(context.Background(), ConvertRequest{
		Source:       src,
		TargetFormat: FormatWebP,
		Output:       dst,
	}))
	last := progress[len(progress)-1]

	if sysdep.Lookup("magick").Found {
		if last.Err != nil {
			t.Fatalf("webp encode via magick failed: %+v", last.Err)
		}
		if _, err := os.Stat(dst); err != nil {
			t.Fatalf("expected %s to be written: %v", dst, err)
		}
	} else if last.Err == nil || last.Err.Code != tools.CodeMissingBinary {
		t.Fatalf("without magick, webp encode should fail MISSING_BINARY, got %+v", last)
	}
}

// writeNamedPNG writes a 4×4 PNG at the given path — used by the batch tests,
// which need sources with distinct basenames.
func writeNamedPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 64), G: uint8(y * 64), B: 200, A: 255})
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

func TestBatchConvertRejectsEmpty(t *testing.T) {
	progress := collect(BatchConvert(context.Background(), BatchConvertRequest{
		TargetFormat: FormatJPEG,
		OutputDir:    t.TempDir(),
	}))
	if last := progress[len(progress)-1]; last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST, got %+v", last)
	}
}

func TestBatchConvertAllSucceed(t *testing.T) {
	dir := t.TempDir()
	out := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	writeNamedPNG(t, a)
	writeNamedPNG(t, b)

	progress := collect(BatchConvert(context.Background(), BatchConvertRequest{
		Sources:      []string{a, b},
		TargetFormat: FormatJPEG,
		OutputDir:    out,
		Opts:         Options{Quality: 80},
	}))

	last := progress[len(progress)-1]
	if !last.Completed || last.Err != nil {
		t.Fatalf("expected success, got %+v", last)
	}
	for _, name := range []string{"a.jpg", "b.jpg"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("output %s missing: %v", name, err)
		}
	}
}

func TestBatchConvertPartialFailureContinues(t *testing.T) {
	dir := t.TempDir()
	out := t.TempDir()
	good := filepath.Join(dir, "good.png")
	writeNamedPNG(t, good)
	missing := filepath.Join(dir, "missing.png")

	progress := collect(BatchConvert(context.Background(), BatchConvertRequest{
		Sources:      []string{good, missing},
		TargetFormat: FormatJPEG,
		OutputDir:    out,
	}))

	// The batch must finish (not abort) and not be marked a hard failure.
	last := progress[len(progress)-1]
	if !last.Completed || last.Err != nil {
		t.Fatalf("expected partial success terminal, got %+v", last)
	}
	if _, err := os.Stat(filepath.Join(out, "good.jpg")); err != nil {
		t.Errorf("good output missing: %v", err)
	}
}

func TestBatchConvertAllFail(t *testing.T) {
	progress := collect(BatchConvert(context.Background(), BatchConvertRequest{
		Sources:      []string{"/nope-1.png", "/nope-2.png"},
		TargetFormat: FormatJPEG,
		OutputDir:    t.TempDir(),
	}))
	if last := progress[len(progress)-1]; last.Err == nil || last.Err.Code != tools.CodeIO {
		t.Fatalf("expected IO_ERROR when every file fails, got %+v", last)
	}
}
