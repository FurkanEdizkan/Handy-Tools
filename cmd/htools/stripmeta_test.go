package main

import (
	"bytes"
	"context"
	stdimage "image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/config"
)

func TestCmdStripMetaUsageErrors(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	cases := []struct {
		name string
		args []string
	}{
		{"no sources", []string{}},
		{"bad flag", []string{"--galaxy", "x.jpg"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := cmdStripMeta(ctx, cfg, tc.args); code == 0 {
				t.Errorf("expected non-zero exit for %s", tc.name)
			}
		})
	}
}

func TestCmdStripMetaUnknownExtension(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if code := cmdStripMeta(context.Background(), config.Defaults(),
		[]string{"--quiet", p}); code != 1 {
		t.Errorf("unknown ext exit = %d, want 1 (file skipped, batch failed)", code)
	}
}

// TestCmdStripMetaRemovesEXIF builds a JPEG with a hand-injected APP1
// segment carrying the canonical "Exif\0\0" marker, runs strip-meta over it,
// and verifies the output JPEG no longer contains that marker. Re-encoding
// through the stdlib jpeg encoder drops every APP segment so this is the
// cheapest "EXIF was actually removed" assertion available without exiftool.
func TestCmdStripMetaRemovesEXIF(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.jpg")
	if err := writeJPEGWithEXIF(src); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Sanity-check that our seed actually has the marker before the strip.
	body, _ := os.ReadFile(src)
	if !bytes.Contains(body, []byte("Exif\x00\x00")) {
		t.Fatalf("test fixture is broken: EXIF marker not present in seed")
	}

	if code := cmdStripMeta(context.Background(), config.Defaults(),
		[]string{"--quiet", src}); code != 0 {
		t.Fatalf("strip-meta exit = %d, want 0", code)
	}
	out := filepath.Join(dir, "photo-stripped.jpg")
	stripped, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read stripped output: %v", err)
	}
	if bytes.Contains(stripped, []byte("Exif\x00\x00")) {
		t.Errorf("output JPEG still contains the EXIF APP1 marker")
	}
	// Re-decoding the output must still work — strip-meta produced a real JPEG.
	if _, err := jpeg.Decode(bytes.NewReader(stripped)); err != nil {
		t.Errorf("output JPEG no longer decodes: %v", err)
	}
}

func TestCmdStripMetaInPlace(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.jpg")
	if err := writeJPEGWithEXIF(src); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if code := cmdStripMeta(context.Background(), config.Defaults(),
		[]string{"--quiet", "--in-place", src}); code != 0 {
		t.Fatalf("strip-meta --in-place exit = %d, want 0", code)
	}
	// The source file itself should no longer have EXIF, and the sibling
	// `-stripped` file should NOT have been created.
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if bytes.Contains(body, []byte("Exif\x00\x00")) {
		t.Errorf("--in-place left EXIF marker in source")
	}
	if _, err := os.Stat(filepath.Join(dir, "photo-stripped.jpg")); err == nil {
		t.Errorf("--in-place should not create a -stripped sibling")
	}
}

func TestCmdStripMetaPNG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "icon.png")
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 2))
	f, err := os.Create(src)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	f.Close()

	if code := cmdStripMeta(context.Background(), config.Defaults(),
		[]string{"--quiet", src}); code != 0 {
		t.Fatalf("strip-meta exit = %d, want 0", code)
	}
	out := filepath.Join(dir, "icon-stripped.png")
	if _, err := os.Stat(out); err != nil {
		t.Errorf("expected %s to exist: %v", out, err)
	}
}

// writeJPEGWithEXIF produces a minimal valid JPEG that has an APP1 "Exif\0\0"
// segment inserted directly after the SOI marker. The payload after the
// marker is fake — JPEG decoders skip unknown bytes inside APP segments, so
// this stays decodable end-to-end.
func writeJPEGWithEXIF(path string) error {
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		return err
	}
	base := buf.Bytes()
	// Build: SOI + APP1 marker + length + "Exif\0\0" + filler + rest of original.
	var out bytes.Buffer
	out.Write(base[:2]) // SOI: FF D8
	out.Write([]byte{0xFF, 0xE1})
	payload := []byte("Exif\x00\x00fake-exif-payload")
	length := 2 + len(payload) // length field itself + payload
	out.WriteByte(byte(length >> 8))
	out.WriteByte(byte(length & 0xff))
	out.Write(payload)
	out.Write(base[2:])
	return os.WriteFile(path, out.Bytes(), 0o644)
}
