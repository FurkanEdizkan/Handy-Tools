package archive_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
	"github.com/furkandedizkan/handy-tools/internal/tools/testutil/stressgen"
)

// BenchmarkCompress packs a small synthetic tree into each pure-Go format.
// The Inspect/Extract benches reuse the artifacts produced here.
func BenchmarkCompress(b *testing.B) {
	src := b.TempDir()
	if _, err := stressgen.ArchiveTree(src, 200, 4<<10, 100, 0xa1); err != nil {
		b.Fatal(err)
	}

	cases := []struct {
		name string
		ext  string
	}{
		{"zip", ".zip"},
		{"tar-gz", ".tar.gz"},
		{"tar-zst", ".tar.zst"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				b.StopTimer()
				out := filepath.Join(b.TempDir(), "archive"+c.ext)
				b.StartTimer()
				for ev := range archive.Compress(ctx, archive.CompressRequest{
					Sources: []string{src},
					Output:  out,
				}) {
					if ev.Err != nil {
						b.Fatalf("Compress %s: %v", c.name, ev.Err)
					}
				}
			}
		})
	}
}

// BenchmarkExtract measures the codepath that perf/archive-zip-single-scan
// and perf/tar-extract-buffer target. We compress once outside the timer,
// then time only the extraction.
func BenchmarkExtract(b *testing.B) {
	cases := []struct {
		name string
		ext  string
	}{
		{"zip", ".zip"},
		{"tar-gz", ".tar.gz"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			src := b.TempDir()
			if _, err := stressgen.ArchiveTree(src, 200, 4<<10, 100, 0xa2); err != nil {
				b.Fatal(err)
			}
			archivePath := filepath.Join(b.TempDir(), "archive"+c.ext)
			ctx := context.Background()
			for ev := range archive.Compress(ctx, archive.CompressRequest{
				Sources: []string{src},
				Output:  archivePath,
			}) {
				if ev.Err != nil {
					b.Fatalf("Compress: %v", ev.Err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				b.StopTimer()
				dest := b.TempDir()
				b.StartTimer()
				for ev := range archive.Extract(ctx, archive.ExtractRequest{
					Source:      archivePath,
					Destination: dest,
					Overwrite:   true,
				}) {
					if ev.Err != nil {
						b.Fatalf("Extract %s: %v", c.name, ev.Err)
					}
				}
			}
		})
	}
}

// BenchmarkInspect measures the cheap preflight that today double-scans
// zip archives (perf/archive-zip-single-scan). Tar formats don't carry a
// central directory so their Inspect is constant-time and uninteresting.
func BenchmarkInspect(b *testing.B) {
	src := b.TempDir()
	if _, err := stressgen.ArchiveTree(src, 500, 4<<10, 100, 0xa3); err != nil {
		b.Fatal(err)
	}
	archivePath := filepath.Join(b.TempDir(), "archive.zip")
	ctx := context.Background()
	for ev := range archive.Compress(ctx, archive.CompressRequest{
		Sources: []string{src},
		Output:  archivePath,
	}) {
		if ev.Err != nil {
			b.Fatalf("Compress: %v", ev.Err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := archive.Inspect(ctx, archivePath); err != nil {
			b.Fatalf("Inspect: %v", err)
		}
	}
}
