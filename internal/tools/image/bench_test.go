package image_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools/image"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
	"github.com/furkandedizkan/handy-tools/internal/tools/testutil/stressgen"
)

// BenchmarkBatchConvert drives image.BatchConvert across the codepaths that
// matter for the perf/image-batch-convert-parallel PR. JPEG -> PNG is the
// pure-Go path; JPEG -> WebP exercises the magick delegation and only runs
// when magick is on PATH so CI without ImageMagick still passes.
func BenchmarkBatchConvert(b *testing.B) {
	src := b.TempDir()
	paths, err := stressgen.ImageSet(src, 20, 512, 0x1)
	if err != nil {
		b.Fatal(err)
	}

	cases := []struct {
		name   string
		format image.Format
		skipIf func() bool
	}{
		{"to-png", image.FormatPNG, nil},
		{"to-webp", image.FormatWebP, func() bool { return !sysdep.Lookup("magick").Found }},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			if c.skipIf != nil && c.skipIf() {
				b.Skip("optional binary not on PATH")
			}
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				b.StopTimer()
				out := b.TempDir() // fresh dir per iter so disambiguatePath doesn't drift
				b.StartTimer()
				for ev := range image.BatchConvert(ctx, image.BatchConvertRequest{
					Sources:      paths,
					TargetFormat: c.format,
					OutputDir:    out,
					Overwrite:    true,
				}) {
					if ev.Err != nil {
						b.Fatalf("BatchConvert: %v", ev.Err)
					}
				}
			}
		})
	}
}

// BenchmarkConvert is the single-file Convert path — the codepath the GUI
// uses when the user drags one image in. Kept small (one 512×512 JPEG) so
// it amortises across many iterations without setup noise.
func BenchmarkConvert(b *testing.B) {
	src := b.TempDir()
	paths, err := stressgen.ImageSet(src, 1, 512, 0x2)
	if err != nil {
		b.Fatal(err)
	}
	source := paths[0]
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		out := filepath.Join(b.TempDir(), "out.png")
		b.StartTimer()
		for ev := range image.Convert(ctx, image.ConvertRequest{
			Source:       source,
			TargetFormat: image.FormatPNG,
			Output:       out,
			Overwrite:    true,
		}) {
			if ev.Err != nil {
				b.Fatalf("Convert: %v", ev.Err)
			}
		}
	}
}
