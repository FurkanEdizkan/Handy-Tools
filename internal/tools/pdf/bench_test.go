package pdf_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools/pdf"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
	"github.com/furkandedizkan/handy-tools/internal/tools/testutil/stressgen"
)

// BenchmarkMerge concatenates 10 small synthetic PDFs via pdfcpu. Pure Go;
// no system binary required.
func BenchmarkMerge(b *testing.B) {
	src := b.TempDir()
	paths, err := stressgen.PDFSet(src, 10, 5, 0)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		out := filepath.Join(b.TempDir(), "merged.pdf")
		b.StartTimer()
		for ev := range pdf.Merge(ctx, pdf.MergeRequest{
			Sources:    paths,
			OutputFile: out,
		}) {
			if ev.Err != nil {
				b.Fatalf("pdf.Merge: %v", ev.Err)
			}
		}
	}
}

// BenchmarkSplit divides a 50-page synthetic PDF into 5-page chunks.
func BenchmarkSplit(b *testing.B) {
	src := b.TempDir()
	source, err := stressgen.PDFLarge(src, 50)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		out := b.TempDir()
		b.StartTimer()
		for ev := range pdf.Split(ctx, pdf.SplitRequest{
			Source:    source,
			EveryN:    5,
			OutputDir: out,
		}) {
			if ev.Err != nil {
				b.Fatalf("pdf.Split: %v", ev.Err)
			}
		}
	}
}

// BenchmarkRender exercises pdftoppm; skips when the binary is absent so
// CI without poppler-utils still passes the bench job.
func BenchmarkRender(b *testing.B) {
	if !sysdep.Lookup("pdftoppm").Found {
		b.Skip("pdftoppm not on PATH")
	}
	src := b.TempDir()
	source, err := stressgen.PDFLarge(src, 20)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		out := b.TempDir()
		b.StartTimer()
		for ev := range pdf.ToImage(ctx, pdf.ToImageRequest{
			Source:    source,
			OutputDir: out,
			DPI:       72,
		}) {
			if ev.Err != nil {
				b.Fatalf("pdf.ToImage: %v", ev.Err)
			}
		}
	}
}

// BenchmarkText exercises pdftotext; skips when the binary is absent.
func BenchmarkText(b *testing.B) {
	if !sysdep.Lookup("pdftotext").Found {
		b.Skip("pdftotext not on PATH")
	}
	src := b.TempDir()
	source, err := stressgen.PDFLarge(src, 50)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		out := filepath.Join(b.TempDir(), "out.txt")
		b.StartTimer()
		for ev := range pdf.ToText(ctx, pdf.ToTextRequest{
			Source:     source,
			OutputFile: out,
		}) {
			if ev.Err != nil {
				b.Fatalf("pdf.ToText: %v", ev.Err)
			}
		}
	}
}
