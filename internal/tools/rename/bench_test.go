package rename_test

import (
	"context"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools/rename"
	"github.com/furkandedizkan/handy-tools/internal/tools/testutil/stressgen"
)

// BenchmarkRun measures the rename pipeline end-to-end: pattern compile,
// collision pre-pass (O(N) with the suffix-resolution loop), and N
// os.Rename syscalls. Workload is small (500 files) because the bench
// regenerates fixtures every iteration — the syscall cost dominates.
func BenchmarkRun(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		dir := b.TempDir()
		paths, err := stressgen.RenameSet(dir, 500)
		if err != nil {
			b.Fatal(err)
		}
		req := rename.Request{
			Sources: paths,
			Pattern: `^(\d+)$`,
			Replace: `file_$1.txt`,
		}
		b.StartTimer()
		for ev := range rename.Run(ctx, req) {
			if ev.Err != nil {
				b.Fatalf("rename.Run: %v", ev.Err)
			}
		}
	}
}

// BenchmarkInspect is the read-only dry-run path — useful as a control for
// BenchmarkRun (how much of the total is regex / collision resolution vs
// syscalls).
func BenchmarkInspect(b *testing.B) {
	dir := b.TempDir()
	paths, err := stressgen.RenameSet(dir, 500)
	if err != nil {
		b.Fatal(err)
	}
	req := rename.Request{
		Sources: paths,
		Pattern: `^(\d+)$`,
		Replace: `file_$1.txt`,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := rename.Inspect(req); err != nil {
			b.Fatalf("rename.Inspect: %v", err)
		}
	}
}
