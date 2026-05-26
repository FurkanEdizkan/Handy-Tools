package hash_test

import (
	"context"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools/hash"
	"github.com/furkandedizkan/handy-tools/internal/tools/testutil/stressgen"
)

// BenchmarkRun exercises hash.Run across all three supported algorithms.
// Workload sized for bench-scale (10 × 1 MB) — the make stress target uses
// the bigger 100 × 10 MB + 1 × 1 GB fixtures from cmd/stressgen.
func BenchmarkRun(b *testing.B) {
	dir := b.TempDir()
	paths, err := stressgen.HashSet(dir, 10, 1<<20, 0xbe)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	for _, algo := range []hash.Algo{hash.AlgoMD5, hash.AlgoSHA256, hash.AlgoBLAKE3} {
		b.Run(string(algo), func(b *testing.B) {
			req := hash.Request{Sources: paths, Algo: algo}
			b.SetBytes(int64(len(paths)) * (1 << 20))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for ev := range hash.Run(ctx, req) {
					if ev.Err != nil {
						b.Fatalf("hash.Run: %v", ev.Err)
					}
				}
			}
		})
	}
}

// BenchmarkRunLarge focuses on the single-large-file streaming path. SHA256
// only — the per-byte cost is the same shape across algorithms and we don't
// need to triple the wall-clock for that.
func BenchmarkRunLarge(b *testing.B) {
	dir := b.TempDir()
	path, err := stressgen.HashLarge(dir, 128<<20, 0xbe) // 128 MB; b.N rescales
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	req := hash.Request{Sources: []string{path}, Algo: hash.AlgoSHA256}
	b.SetBytes(128 << 20)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for ev := range hash.Run(ctx, req) {
			if ev.Err != nil {
				b.Fatalf("hash.Run: %v", ev.Err)
			}
		}
	}
}
