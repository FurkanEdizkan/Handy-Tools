package stripmeta

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/tiff"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// TestRunInPlaceEncodeFailureKeepsSource: without --rollback-on-error there
// is no .handy-bak snapshot, so a failed re-encode must not have truncated
// the source first. The fixture is a PNG too wide for the JPEG encoder,
// saved under a .jpg name: it decodes, then every JPEG encode fails.
func TestRunInPlaceEncodeFailureKeepsSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "wide.jpg")
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 70000, 1))); err != nil {
		t.Fatal(err)
	}
	original := buf.Bytes()
	if err := os.WriteFile(src, original, 0o644); err != nil {
		t.Fatal(err)
	}

	last := terminal(t, collect(Run(context.Background(), Request{Sources: []string{src}, InPlace: true})))
	if last.Err == nil {
		t.Fatalf("expected the re-encode to fail, got success: %+v", last)
	}
	if got, _ := os.ReadFile(src); !bytes.Equal(got, original) {
		t.Fatalf("source destroyed by a failed in-place strip (now %d bytes)", len(got))
	}
}

// TestRunInPlaceKeepsAnimatedGIFFrames: stripping an animated GIF must keep
// every frame, not flatten it to the first one.
func TestRunInPlaceKeepsAnimatedGIFFrames(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "anim.gif")
	g := &gif.GIF{}
	for i := 0; i < 3; i++ {
		fr := image.NewPaletted(image.Rect(0, 0, 4, 4), palette.Plan9)
		fr.Set(i, i, color.White)
		g.Image = append(g.Image, fr)
		g.Delay = append(g.Delay, 10)
	}
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(f, g); err != nil {
		t.Fatal(err)
	}
	f.Close()

	last := terminal(t, collect(Run(context.Background(), Request{Sources: []string{src}, InPlace: true})))
	if last.Err != nil || len(last.Failures) != 0 {
		t.Fatalf("strip: %+v", last)
	}
	f, err = os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Image) != 3 {
		t.Fatalf("frames after strip = %d, want 3", len(out.Image))
	}
}

// TestRunMultiPageTIFFRefused: the TIFF decoder only returns page one, so a
// multi-page TIFF is reported UNSUPPORTED_INPUT and left untouched.
func TestRunMultiPageTIFFRefused(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "scan.tif")
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes() // little-endian ("II") from the x/image encoder
	off := binary.LittleEndian.Uint32(b[4:8])
	n := uint32(binary.LittleEndian.Uint16(b[off : off+2]))
	ifdLen := 2 + 12*n + 4
	if len(b)%2 == 1 {
		b = append(b, 0)
	}
	second := uint32(len(b))
	b = append(b, b[off:off+ifdLen]...)
	binary.LittleEndian.PutUint32(b[second+ifdLen-4:], 0)
	binary.LittleEndian.PutUint32(b[off+2+12*n:], second)
	if err := os.WriteFile(src, b, 0o644); err != nil {
		t.Fatal(err)
	}

	last := terminal(t, collect(Run(context.Background(), Request{Sources: []string{src}, InPlace: true})))
	if last.Err == nil || last.Err.Code != tools.CodeUnsupportedInput {
		t.Fatalf("want UNSUPPORTED_INPUT, got %+v", last.Err)
	}
	if got, _ := os.ReadFile(src); !bytes.Equal(got, b) {
		t.Fatalf("multi-page TIFF was modified")
	}
}

// TestRunCancelWithRollbackRestoresSources: cancelling an in-place
// --rollback-on-error batch must restore the files already rewritten and
// leave no .handy-bak snapshots behind — not only a per-file failure.
func TestRunCancelWithRollbackRestoresSources(t *testing.T) {
	dir := t.TempDir()
	originals := map[string][]byte{}
	var srcs []string
	for i := 0; i < 40; i++ {
		p := filepath.Join(dir, fmt.Sprintf("f%02d.png", i))
		img := image.NewRGBA(image.Rect(0, 0, 16, 16))
		img.Set(i%16, 0, color.RGBA{R: 255, A: 255})
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		// Trailing bytes after IEND survive a byte comparison only if the
		// file was restored rather than re-encoded.
		data := append(buf.Bytes(), []byte("original-trailer")...)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		originals[p] = data
		srcs = append(srcs, p)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n := 0
	for range Run(ctx, Request{Sources: srcs, InPlace: true, Rollback: true}) {
		n++
		if n == 2 {
			cancel()
		}
	}

	baks, _ := filepath.Glob(filepath.Join(dir, "*.handy-bak"))
	if len(baks) != 0 {
		t.Errorf("cancel left %d .handy-bak snapshot(s) behind: %v", len(baks), baks)
	}
	for p, want := range originals {
		if got, _ := os.ReadFile(p); !bytes.Equal(got, want) {
			t.Errorf("%s was not restored after cancel", filepath.Base(p))
		}
	}
}
