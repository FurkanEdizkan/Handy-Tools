// Package stressgen produces deterministic synthetic fixtures used by the
// repo's stress harness (cmd/stressgen) and per-package benchmarks
// (internal/tools/<pkg>/bench_test.go).
//
// Every generator function takes an output directory plus a seed; the same
// seed always produces byte-identical output so before/after benchstat runs
// are comparable. Fixtures are written eagerly — generators block until disk
// state matches the spec — and are *not* cleaned up: callers (tests via
// b.TempDir, the harness via the operator) own that.
//
// The package has no test dependency on the tools/* packages so it can be
// imported by their benches without an import cycle.
package stressgen

import (
	"fmt"
	"os"
	"syscall"
)

// MinFreeBytes is the disk-free floor every generator checks before writing
// anything large (>1 MB). It exists to keep an accidentally-too-large spec
// from filling the operator's /tmp.
const MinFreeBytes = 10 << 30 // 10 GiB

// EnsureFreeSpace returns ErrInsufficientSpace when the filesystem holding
// dir has less than MinFreeBytes available. Generators call it before
// writing any large workload. Best-effort: if the syscall fails (e.g. on a
// platform we don't support), the check passes — production runs are on
// Linux and we don't want the harness to silently skip on a stat failure.
func EnsureFreeSpace(dir string) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return nil
	}
	free := st.Bavail * uint64(st.Bsize) //nolint:gosec // values fit
	if free < MinFreeBytes {
		return fmt.Errorf("stressgen: need at least %d GiB free at %s, have %d MiB",
			MinFreeBytes>>30, dir, free>>20)
	}
	return nil
}

// MkdirAll is os.MkdirAll wrapped so the error includes the path — the
// generators chain a lot of dir creation and a bare "permission denied"
// without a path is unhelpful in CI logs.
func MkdirAll(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return nil
}

// Workload names the canonical set of stressgen workloads. The harness CLI
// uses these as the --workload value; bench files use them only for
// documentation. Keep this list in sync with the Generate dispatch below.
const (
	WorkloadHashSet        = "hash-set"
	WorkloadHashLarge      = "hash-large"
	WorkloadImageSet       = "image-set"
	WorkloadArchiveTree    = "archive-tree"
	WorkloadArchiveLarge   = "archive-large"
	WorkloadPDFSet         = "pdf-set"
	WorkloadPDFLarge       = "pdf-large"
	WorkloadDiffTreesMTime = "difftrees-mtime"
	WorkloadDiffTreesHash  = "difftrees-hash"
	WorkloadRenameSet      = "rename-set"
)

// Workloads is the canonical workload list. The harness uses it for --list.
var Workloads = []string{
	WorkloadHashSet, WorkloadHashLarge,
	WorkloadImageSet,
	WorkloadArchiveTree, WorkloadArchiveLarge,
	WorkloadPDFSet, WorkloadPDFLarge,
	WorkloadDiffTreesMTime, WorkloadDiffTreesHash,
	WorkloadRenameSet,
}
