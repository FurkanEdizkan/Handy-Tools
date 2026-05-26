// stressgen materialises the synthetic fixtures the repo's stress harness
// drives the CLI and MCP binaries against. It is a thin dispatcher over
// internal/tools/testutil/stressgen — no logic of its own. Built by
// `make stress` and not part of the user-facing release.
//
// Usage:
//
//	stressgen --workload=hash-set       --dir=/tmp/handy-stress/hash-set
//	stressgen --workload=archive-tree   --dir=/tmp/handy-stress/archive-tree
//	stressgen --list
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/furkandedizkan/handy-tools/internal/tools/testutil/stressgen"
)

func main() {
	workload := flag.String("workload", "", "name of the workload to generate (see --list)")
	dir := flag.String("dir", "", "destination directory (must not be empty)")
	list := flag.Bool("list", false, "print the canonical workload list and exit")
	seed := flag.Uint64("seed", 0x68616e6479, "PRNG seed; same seed produces byte-identical output")
	flag.Parse()

	if *list {
		for _, w := range stressgen.Workloads {
			fmt.Println(w)
		}
		return
	}
	if *workload == "" || *dir == "" {
		fmt.Fprintln(os.Stderr, "stressgen: --workload and --dir are required (use --list to see workloads)")
		os.Exit(2)
	}

	abs, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stressgen:", err)
		os.Exit(1)
	}

	if err := run(*workload, abs, *seed); err != nil {
		fmt.Fprintln(os.Stderr, "stressgen:", err)
		os.Exit(1)
	}
}

func run(workload, dir string, seed uint64) error {
	switch workload {
	case stressgen.WorkloadHashSet:
		_, err := stressgen.HashSet(dir, 100, 10<<20, seed)
		return err
	case stressgen.WorkloadHashLarge:
		_, err := stressgen.HashLarge(dir, 1<<30, seed)
		return err
	case stressgen.WorkloadImageSet:
		_, err := stressgen.ImageSet(dir, 500, 1024, seed)
		return err
	case stressgen.WorkloadArchiveTree:
		_, err := stressgen.ArchiveTree(dir, 10000, 4<<10, 100, seed)
		return err
	case stressgen.WorkloadArchiveLarge:
		_, err := stressgen.ArchiveLarge(dir, 1<<30, seed)
		return err
	case stressgen.WorkloadPDFSet:
		_, err := stressgen.PDFSet(dir, 20, 10, seed)
		return err
	case stressgen.WorkloadPDFLarge:
		_, err := stressgen.PDFLarge(dir, 200)
		return err
	case stressgen.WorkloadDiffTreesMTime:
		_, _, err := stressgen.DiffTrees(dir, 50000, 256, 1, seed)
		return err
	case stressgen.WorkloadDiffTreesHash:
		_, _, err := stressgen.DiffTrees(dir, 5000, 4<<10, 1, seed)
		return err
	case stressgen.WorkloadRenameSet:
		_, err := stressgen.RenameSet(dir, 10000)
		return err
	}
	return fmt.Errorf("unknown workload %q (use --list)", workload)
}
