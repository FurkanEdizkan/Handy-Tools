package difftree_test

import (
	"context"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools/difftree"
	"github.com/furkandedizkan/handy-tools/internal/tools/testutil/stressgen"
)

// BenchmarkRun exercises both modes. ModeMTime is the cheap walk; ModeHash
// is what perf/difftree-hash-parallel targets — it reads every byte of every
// file on each side, so its wins should be measurable in throughput.
func BenchmarkRun(b *testing.B) {
	root := b.TempDir()
	rootA, rootB, err := stressgen.DiffTrees(root, 500, 4<<10, 1, 0xd1)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	for _, mode := range []difftree.Mode{difftree.ModeMTime, difftree.ModeHash} {
		b.Run(string(mode), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for ev := range difftree.Run(ctx, difftree.Request{
					A: rootA, B: rootB, Mode: mode,
				}) {
					if ev.Err != nil {
						b.Fatalf("difftree.Run: %v", ev.Err)
					}
				}
			}
		})
	}
}
