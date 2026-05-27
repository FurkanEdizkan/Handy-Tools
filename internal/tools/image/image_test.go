package image

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

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

func TestConvertSuffixesOnCollision(t *testing.T) {
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "out.png")
	original := []byte("preexisting")
	if err := os.WriteFile(dst, original, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	progress := collect(Convert(context.Background(), ConvertRequest{
		Source:       src,
		TargetFormat: FormatPNG,
		Output:       dst,
	}))
	last := progress[len(progress)-1]
	if !last.Completed || last.Err != nil {
		t.Fatalf("expected success, got %+v", last)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("re-read original: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("original out.png was overwritten (Overwrite=false should preserve it)")
	}

	first := filepath.Join(dir, "out-1.png")
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("expected suffixed output %s: %v", first, err)
	}

	// Second collision: out.png AND out-1.png exist, so the next conversion
	// should walk the counter forward to out-2.png.
	progress = collect(Convert(context.Background(), ConvertRequest{
		Source:       src,
		TargetFormat: FormatPNG,
		Output:       dst,
	}))
	last = progress[len(progress)-1]
	if !last.Completed || last.Err != nil {
		t.Fatalf("expected success on second collision, got %+v", last)
	}
	second := filepath.Join(dir, "out-2.png")
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("expected counter to advance to %s: %v", second, err)
	}
}

func TestConvertOverwriteClobbersInPlace(t *testing.T) {
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
		Overwrite:    true,
	}))
	last := progress[len(progress)-1]
	if !last.Completed || last.Err != nil {
		t.Fatalf("expected success, got %+v", last)
	}

	// With Overwrite=true we must NOT have created a suffixed sibling — the
	// original path is clobbered with the new PNG contents.
	if _, err := os.Stat(filepath.Join(dir, "out-1.png")); !os.IsNotExist(err) {
		t.Fatalf("Overwrite=true should not produce a suffixed sibling, got err=%v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read clobbered: %v", err)
	}
	if string(got) == "preexisting" {
		t.Fatalf("Overwrite=true did not replace the original file")
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
	last := progress[len(progress)-1]
	// Every file failed with the same NOT_FOUND classification, so the
	// terminal Code should coalesce to that — not collapse to IO_ERROR.
	if last.Err == nil || last.Err.Code != tools.CodeNotFound {
		t.Fatalf("expected NOT_FOUND when every file vanishes, got %+v", last)
	}
	if len(last.Failures) != 2 {
		t.Errorf("expected 2 failure entries on terminal event, got %d: %+v", len(last.Failures), last.Failures)
	}
}

// writeTinyImage encodes a 4×4 RGBA fixture in the requested source format
// at dst. Matrix-test helper — keeps each fixture tiny so the suite stays
// fast. Returns the path it wrote.
func writeTinyImage(t *testing.T, dst string, srcFmt Format) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 64), G: uint8(y * 64), B: 100, A: 255})
		}
	}
	f, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer f.Close()
	switch srcFmt {
	case FormatPNG:
		err = png.Encode(f, img)
	case FormatJPEG:
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	case FormatGIF:
		err = gif.Encode(f, img, nil)
	case FormatBMP:
		err = bmp.Encode(f, img)
	case FormatTIFF:
		err = tiff.Encode(f, img, nil)
	default:
		t.Fatalf("writeTinyImage: unsupported source format %v", srcFmt)
	}
	if err != nil {
		t.Fatalf("encode %s: %v", dst, err)
	}
	return dst
}

// TestConvertMatrix exercises every native (source × target) combination so
// no codec path silently breaks. WebP/HEIC targets are gated on ImageMagick
// being on PATH — the test sub-skips cleanly if it isn't, matching the
// existing TestWebPEncodeViaMagick pattern.
func TestConvertMatrix(t *testing.T) {
	sysdep.Reset()
	hasMagick := sysdep.Lookup("magick").Found

	type cell struct {
		name  string
		src   Format
		dst   Format
		magic bool // requires ImageMagick (WebP / HEIC encode)
	}
	srcs := []Format{FormatPNG, FormatJPEG, FormatGIF, FormatBMP, FormatTIFF}
	dsts := []struct {
		f     Format
		magic bool
	}{
		{FormatPNG, false},
		{FormatJPEG, false},
		{FormatGIF, false},
		{FormatBMP, false},
		{FormatTIFF, false},
		{FormatWebP, true},
		{FormatHEIC, true},
	}

	cells := make([]cell, 0, len(srcs)*len(dsts))
	for _, s := range srcs {
		for _, d := range dsts {
			// Identity is exercised by TestConvertResizeShrinks; matrix
			// focuses on format-translating paths.
			if s == d.f {
				continue
			}
			cells = append(cells, cell{
				name:  fmt.Sprintf("%s_to_%s", s.Ext()[1:], d.f.Ext()[1:]),
				src:   s,
				dst:   d.f,
				magic: d.magic,
			})
		}
	}

	for _, c := range cells {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if c.magic && !hasMagick {
				t.Skipf("ImageMagick not on PATH; skipping %s", c.name)
			}
			dir := t.TempDir()
			src := writeTinyImage(t, filepath.Join(dir, "in"+c.src.Ext()), c.src)
			out := filepath.Join(dir, "out"+c.dst.Ext())
			progress := collect(Convert(context.Background(), ConvertRequest{
				Source:       src,
				TargetFormat: c.dst,
				Output:       out,
				Opts:         Options{Quality: 80},
			}))
			last := progress[len(progress)-1]
			if !last.Completed || last.Err != nil {
				t.Fatalf("%s failed: %+v", c.name, last)
			}
			info, err := os.Stat(out)
			if err != nil {
				t.Fatalf("%s: output missing: %v", c.name, err)
			}
			if info.Size() == 0 {
				t.Fatalf("%s: output is empty", c.name)
			}
		})
	}
}
