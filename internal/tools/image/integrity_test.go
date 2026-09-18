package image

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/tiff"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// withoutMagick hides every binary from sysdep for the duration of the test.
func withoutMagick(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", "")
	sysdep.Reset()
	t.Cleanup(sysdep.Reset)
}

func terminalOf(t *testing.T, ch <-chan tools.Progress) tools.Progress {
	t.Helper()
	events := collect(ch)
	last := events[len(events)-1]
	if !last.Completed {
		t.Fatalf("stream ended without a Completed event: %+v", last)
	}
	return last
}

// assertNoTempFiles fails if a .handy-tmp sibling survived in dir.
func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".handy-tmp-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// TestConvertFailedEncodeKeepsExistingOutput: a WebP convert without
// ImageMagick must fail MISSING_BINARY without first truncating the file it
// was told to overwrite.
func TestConvertFailedEncodeKeepsExistingOutput(t *testing.T) {
	withoutMagick(t)
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "keep.webp")
	original := []byte("existing webp bytes")
	if err := os.WriteFile(dst, original, 0o644); err != nil {
		t.Fatal(err)
	}

	last := terminalOf(t, Convert(context.Background(), ConvertRequest{
		Source: src, TargetFormat: FormatWebP, Output: dst, Overwrite: true,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeMissingBinary {
		t.Fatalf("want MISSING_BINARY, got %+v", last.Err)
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, original) {
		t.Fatalf("existing output was modified by a failed encode: %q", got)
	}
	assertNoTempFiles(t, dir)
}

// TestConvertFailedEncodeLeavesNoPlaceholder: without Overwrite the output
// name is reserved up front; a failed encode must release it so a retry gets
// the natural name rather than a -1 suffix.
func TestConvertFailedEncodeLeavesNoPlaceholder(t *testing.T) {
	withoutMagick(t)
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)

	last := terminalOf(t, Convert(context.Background(), ConvertRequest{
		Source: src, TargetFormat: FormatWebP,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeMissingBinary {
		t.Fatalf("want MISSING_BINARY, got %+v", last.Err)
	}
	if _, err := os.Stat(filepath.Join(dir, "in.webp")); !os.IsNotExist(err) {
		t.Fatalf("failed encode left an output placeholder behind (stat err=%v)", err)
	}
	assertNoTempFiles(t, dir)
}

// writeWidePNGAsJPEG writes a PNG too wide for the JPEG encoder under a .jpg
// name: it decodes fine but every JPEG encode of it fails, which makes the
// encode-failure path deterministic.
func writeWidePNGAsJPEG(t *testing.T, path string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 70000, 1))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestConvertEncodeFailureInPlaceKeepsSource: overwriting the source itself
// (what strip-meta --in-place does) must leave it intact when encoding fails.
func TestConvertEncodeFailureInPlaceKeepsSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "wide.jpg")
	original := writeWidePNGAsJPEG(t, src)

	last := terminalOf(t, Convert(context.Background(), ConvertRequest{
		Source: src, TargetFormat: FormatJPEG, Output: src, Overwrite: true,
	}))
	if last.Err == nil {
		t.Fatalf("expected the JPEG encode to fail, got success")
	}
	if got, _ := os.ReadFile(src); !bytes.Equal(got, original) {
		t.Fatalf("source was destroyed by a failed in-place encode (now %d bytes)", len(got))
	}
	assertNoTempFiles(t, dir)
}

// TestConvertOverwritePreservesMode: replacing an existing file keeps its
// permission bits, as truncating it in place used to.
func TestConvertOverwritePreservesMode(t *testing.T) {
	dir := t.TempDir()
	src := writeTinyPNG(t, dir)
	dst := filepath.Join(dir, "out.png")
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	last := terminalOf(t, Convert(context.Background(), ConvertRequest{
		Source: src, TargetFormat: FormatPNG, Output: dst, Overwrite: true,
	}))
	if last.Err != nil {
		t.Fatalf("convert: %+v", last.Err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

// TestHEICWithoutMagickIsMissingBinary: the decode path must keep the
// structured MISSING_BINARY error instead of collapsing it to IO_ERROR.
func TestHEICWithoutMagickIsMissingBinary(t *testing.T) {
	withoutMagick(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.heic")
	if err := os.WriteFile(src, []byte("not decoded without magick"), 0o644); err != nil {
		t.Fatal(err)
	}

	last := terminalOf(t, Convert(context.Background(), ConvertRequest{Source: src, TargetFormat: FormatPNG}))
	if last.Err == nil || last.Err.Code != tools.CodeMissingBinary {
		t.Errorf("Convert: want MISSING_BINARY, got %+v", last.Err)
	}

	last = terminalOf(t, BatchConvert(context.Background(), BatchConvertRequest{
		Sources: []string{src}, TargetFormat: FormatPNG, OutputDir: dir,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeMissingBinary {
		t.Errorf("BatchConvert: want MISSING_BINARY, got %+v", last.Err)
	}

	_, err := Preview(PreviewRequest{Source: src})
	var te *tools.Error
	if !errors.As(err, &te) || te.Code != tools.CodeMissingBinary {
		t.Errorf("Preview: want MISSING_BINARY, got %v", err)
	}
}

// writeAnimatedGIF writes an n-frame GIF and returns its path.
func writeAnimatedGIF(t *testing.T, path string, n int) {
	t.Helper()
	g := &gif.GIF{LoopCount: 0}
	for i := 0; i < n; i++ {
		fr := image.NewPaletted(image.Rect(0, 0, 4, 4), palette.Plan9)
		fr.Set(i%4, i%4, color.White)
		g.Image = append(g.Image, fr)
		g.Delay = append(g.Delay, 10*(i+1))
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := gif.EncodeAll(f, g); err != nil {
		t.Fatal(err)
	}
}

func gifFrames(t *testing.T, path string) *gif.GIF {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return g
}

// TestConvertGIFToGIFKeepsAllFrames: a GIF→GIF re-encode (strip-meta, or an
// in-place overwrite) must keep every frame and its timing.
func TestConvertGIFToGIFKeepsAllFrames(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "anim.gif")
	writeAnimatedGIF(t, src, 3)

	last := terminalOf(t, Convert(context.Background(), ConvertRequest{
		Source: src, TargetFormat: FormatGIF, Output: src, Overwrite: true,
	}))
	if last.Err != nil {
		t.Fatalf("convert: %+v", last.Err)
	}
	g := gifFrames(t, src)
	if len(g.Image) != 3 {
		t.Fatalf("frames = %d, want 3", len(g.Image))
	}
	if g.Delay[2] != 30 {
		t.Errorf("frame delay not preserved: %v", g.Delay)
	}
}

// TestConvertAnimatedGIFResizeUnsupported: resizing would have to flatten the
// animation, so it is refused rather than silently dropping frames.
func TestConvertAnimatedGIFResizeUnsupported(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "anim.gif")
	writeAnimatedGIF(t, src, 2)
	last := terminalOf(t, Convert(context.Background(), ConvertRequest{
		Source: src, TargetFormat: FormatGIF, Output: filepath.Join(dir, "small.gif"),
		Opts: Options{MaxWidth: 2},
	}))
	if last.Err == nil || last.Err.Code != tools.CodeUnsupportedInput {
		t.Fatalf("want UNSUPPORTED_INPUT, got %+v", last.Err)
	}
}

// writeTwoPageTIFF builds a TIFF whose first IFD chains to a second one.
func writeTwoPageTIFF(t *testing.T, path string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if string(b[:2]) != "II" {
		t.Fatalf("expected little-endian TIFF from the encoder, got %q", b[:2])
	}
	off := binary.LittleEndian.Uint32(b[4:8])
	n := binary.LittleEndian.Uint16(b[off : off+2])
	ifdLen := 2 + 12*uint32(n) + 4
	if len(b)%2 == 1 {
		b = append(b, 0)
	}
	second := uint32(len(b))
	b = append(b, b[off:off+ifdLen]...)
	binary.LittleEndian.PutUint32(b[second+ifdLen-4:], 0)
	binary.LittleEndian.PutUint32(b[off+2+12*uint32(n):], second)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return b
}

// TestConvertMultiPageTIFFToTIFFUnsupported: re-encoding a multi-page TIFF as
// TIFF would keep only page one, so it is refused.
func TestConvertMultiPageTIFFToTIFFUnsupported(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "scan.tiff")
	original := writeTwoPageTIFF(t, src)
	last := terminalOf(t, Convert(context.Background(), ConvertRequest{
		Source: src, TargetFormat: FormatTIFF, Output: src, Overwrite: true,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeUnsupportedInput {
		t.Fatalf("want UNSUPPORTED_INPUT, got %+v", last.Err)
	}
	if got, _ := os.ReadFile(src); !bytes.Equal(got, original) {
		t.Fatalf("multi-page TIFF was modified")
	}
}

// writeAPNG writes a PNG carrying an acTL chunk (the APNG animation marker).
func writeAPNG(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	// signature + IHDR chunk (length/type header, 13 data bytes, crc)
	const ihdrEnd = 8 + 8 + 13 + 4
	// acTL payload: num_frames=2, num_plays=0
	data := []byte{0, 0, 0, 2, 0, 0, 0, 0}
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	chunk = append(chunk, "acTL"...)
	chunk = append(chunk, data...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(append([]byte("acTL"), data...)))
	out := append(append(append([]byte{}, b[:ihdrEnd]...), chunk...), b[ihdrEnd:]...)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConvertAPNGToPNGUnsupported(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "sticker.png")
	writeAPNG(t, src)
	last := terminalOf(t, Convert(context.Background(), ConvertRequest{
		Source: src, TargetFormat: FormatPNG, Output: src, Overwrite: true,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeUnsupportedInput {
		t.Fatalf("want UNSUPPORTED_INPUT, got %+v", last.Err)
	}
}

// TestResizeNeverProducesZeroDimension: extreme aspect ratios used to round a
// side down to 0, yielding an image no encoder accepts.
func TestResizeNeverProducesZeroDimension(t *testing.T) {
	got := resize(image.NewRGBA(image.Rect(0, 0, 1000, 3)), 320, 320)
	if b := got.Bounds(); b.Dx() != 320 || b.Dy() != 1 {
		t.Fatalf("resize 1000x3 into 320x320 = %dx%d, want 320x1", b.Dx(), b.Dy())
	}
	got = resize(image.NewRGBA(image.Rect(0, 0, 3, 1000)), 0, 100)
	if b := got.Bounds(); b.Dx() != 1 || b.Dy() != 100 {
		t.Fatalf("resize 3x1000 into h=100 = %dx%d, want 1x100", b.Dx(), b.Dy())
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "strip.png")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 1000, 3))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	res, err := Preview(PreviewRequest{Source: src})
	if err != nil {
		t.Fatalf("preview of a 1000x3 image failed: %v", err)
	}
	if res.Width != DefaultPreviewBound || res.Height != 1 {
		t.Fatalf("preview = %dx%d, want %dx1", res.Width, res.Height, DefaultPreviewBound)
	}
}

// TestBatchConvertOverwriteDuplicateBasenames: two sources that map to the
// same output name must both survive; the first in source order keeps the
// natural name and the second is suffixed like the non-overwrite path does.
func TestBatchConvertOverwriteDuplicateBasenames(t *testing.T) {
	base := t.TempDir()
	d1, d2, out := filepath.Join(base, "d1"), filepath.Join(base, "d2"), filepath.Join(base, "out")
	for _, d := range []string{d1, d2, out} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeSizedPNG := func(p string, w int) {
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, w, w))); err != nil {
			t.Fatal(err)
		}
	}
	writeSizedPNG(filepath.Join(d1, "a.png"), 6)
	writeSizedPNG(filepath.Join(d2, "a.png"), 3)
	// A third source whose natural output is the first suffix name: the
	// duplicate must skip it rather than race it.
	writeSizedPNG(filepath.Join(d1, "a-1.png"), 5)

	last := terminalOf(t, BatchConvert(context.Background(), BatchConvertRequest{
		Sources:      []string{filepath.Join(d1, "a.png"), filepath.Join(d2, "a.png"), filepath.Join(d1, "a-1.png")},
		TargetFormat: FormatGIF, OutputDir: out, Overwrite: true, Parallelism: 3,
	}))
	if last.Err != nil || len(last.Failures) != 0 {
		t.Fatalf("batch: %+v", last)
	}
	width := func(p string) int {
		f, err := os.Open(p)
		if err != nil {
			t.Fatalf("expected output %s: %v", p, err)
		}
		defer f.Close()
		cfg, err := gif.DecodeConfig(f)
		if err != nil {
			t.Fatalf("decode %s: %v", p, err)
		}
		return cfg.Width
	}
	if w := width(filepath.Join(out, "a.gif")); w != 6 {
		t.Errorf("a.gif should come from the first source (6px), got %dpx", w)
	}
	if w := width(filepath.Join(out, "a-1.gif")); w != 5 {
		t.Errorf("a-1.gif should come from a-1.png (5px), got %dpx", w)
	}
	if w := width(filepath.Join(out, "a-2.gif")); w != 3 {
		t.Errorf("a-2.gif should come from the duplicate d2/a.png (3px), got %dpx", w)
	}
}

// TestBatchConvertCreatesMissingOutputDir: a not-yet-existing OutputDir is a
// directory to create, not a file name every output overwrites.
func TestBatchConvertCreatesMissingOutputDir(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png")
	writeNamedPNG(t, a)
	writeNamedPNG(t, b)
	out := filepath.Join(dir, "new", "converted")

	ins, terr := InspectBatch(BatchConvertRequest{Sources: []string{a, b}, OutputDir: out})
	if terr != nil || len(ins.Issues) != 0 {
		t.Fatalf("preflight should accept a creatable output dir, got %v / %+v", terr, ins.Issues)
	}

	last := terminalOf(t, BatchConvert(context.Background(), BatchConvertRequest{
		Sources: []string{a, b}, TargetFormat: FormatJPEG, OutputDir: out,
	}))
	if last.Err != nil {
		t.Fatalf("batch: %+v", last.Err)
	}
	for _, name := range []string{"a.jpg", "b.jpg"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("expected %s inside the created output dir: %v", name, err)
		}
	}
}

func TestBatchConvertOutputDirIsAFile(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	writeNamedPNG(t, a)
	out := filepath.Join(dir, "notadir")
	if err := os.WriteFile(out, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	last := terminalOf(t, BatchConvert(context.Background(), BatchConvertRequest{
		Sources: []string{a}, TargetFormat: FormatJPEG, OutputDir: out, Overwrite: true,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("want BAD_REQUEST, got %+v", last.Err)
	}
	if got, _ := os.ReadFile(out); string(got) != "keep" {
		t.Fatalf("file passed as OutputDir was overwritten")
	}
}
