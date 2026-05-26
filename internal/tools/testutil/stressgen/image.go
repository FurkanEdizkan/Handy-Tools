package stressgen

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand/v2"
	"os"
	"path/filepath"
)

// ImageSet writes count JPEG files of dims×dims into dir, each filled with
// deterministic random colour noise. Used for image.BatchConvert and
// image.StripMeta benches. Returns absolute paths in deterministic order.
//
// JPEGs are decoded from disk on every bench iteration, so the file content
// must be valid JPEG bytes — we can't shortcut by writing raw RGB.
func ImageSet(dir string, count, dims int, seed uint64) ([]string, error) {
	if err := MkdirAll(dir); err != nil {
		return nil, err
	}
	paths := make([]string, count)
	// Sanity floor on the disk-free check — 500 × 1 MP JPEGs land near
	// ~150 MB and we want the harness to fail loudly before filling /tmp.
	if int64(count)*int64(dims)*int64(dims)*3/4 > 1<<20 {
		if err := EnsureFreeSpace(dir); err != nil {
			return nil, err
		}
	}
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("img_%05d.jpg", i))
		paths[i] = path
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			continue
		}
		if err := writeRandomJPEG(path, dims, seed, uint64(i)); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func writeRandomJPEG(path string, dims int, seed, idx uint64) error {
	r := rand.New(rand.NewPCG(seed, idx)) //nolint:gosec // deterministic fixture
	img := image.NewRGBA(image.Rect(0, 0, dims, dims))
	// Generate large flat blocks so JPEG compression has something to work
	// with — pure-random pixels defeat the encoder and produce 2-3 MB files
	// instead of ~300 KB. Block size 16 matches the JPEG MCU.
	const block = 16
	for by := 0; by < dims; by += block {
		for bx := 0; bx < dims; bx += block {
			v := r.Uint32()
			c := color.RGBA{byte(v), byte(v >> 8), byte(v >> 16), 255}
			for y := by; y < by+block && y < dims; y++ {
				for x := bx; x < bx+block && x < dims; x++ {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
	f, err := os.Create(path) //nolint:gosec
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 85}); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}
