package stressgen

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
)

// DiffTrees writes two sibling trees rooted at dir/A and dir/B, each with
// `files` regular files (`fileSize` bytes apiece). Approximately churnPct
// percent of files differ between the two: the remaining files are byte-
// identical with identical mtimes, so difftree's MTime mode catches only
// the churned set.
//
// Returns (rootA, rootB).
func DiffTrees(dir string, files, fileSize, churnPct int, seed uint64) (string, string, error) {
	if err := MkdirAll(dir); err != nil {
		return "", "", err
	}
	rootA := filepath.Join(dir, "A")
	rootB := filepath.Join(dir, "B")
	if int64(files)*int64(fileSize)*2 > 1<<20 {
		if err := EnsureFreeSpace(dir); err != nil {
			return "", "", err
		}
	}
	if err := MkdirAll(rootA); err != nil {
		return "", "", err
	}
	if err := MkdirAll(rootB); err != nil {
		return "", "", err
	}

	churnEvery := 100
	if churnPct > 0 {
		churnEvery = 100 / churnPct
		if churnEvery == 0 {
			churnEvery = 1
		}
	}

	for i := 0; i < files; i++ {
		sub := i / 100
		subA := filepath.Join(rootA, fmt.Sprintf("d%04d", sub))
		subB := filepath.Join(rootB, fmt.Sprintf("d%04d", sub))
		if i%100 == 0 {
			if err := MkdirAll(subA); err != nil {
				return "", "", err
			}
			if err := MkdirAll(subB); err != nil {
				return "", "", err
			}
		}
		name := fmt.Sprintf("f%05d.bin", i)
		pathA := filepath.Join(subA, name)
		pathB := filepath.Join(subB, name)

		// Skip when both ends already match what we'd write.
		if needsRewrite(pathA, int64(fileSize)) || needsRewrite(pathB, int64(fileSize)) {
			rA := rand.New(rand.NewPCG(seed, uint64(i))) //nolint:gosec // deterministic
			if err := writeRandomFile(pathA, int64(fileSize), rA); err != nil {
				return "", "", err
			}
			if i%churnEvery == 0 {
				// Churned: B gets a different seed.
				rB := rand.New(rand.NewPCG(seed^0xdead, uint64(i))) //nolint:gosec
				if err := writeRandomFile(pathB, int64(fileSize), rB); err != nil {
					return "", "", err
				}
			} else {
				// Unchanged: byte-identical, and we copy mtime so MTime
				// mode reports the file as unchanged.
				rB := rand.New(rand.NewPCG(seed, uint64(i))) //nolint:gosec
				if err := writeRandomFile(pathB, int64(fileSize), rB); err != nil {
					return "", "", err
				}
				if err := copyMTime(pathA, pathB); err != nil {
					return "", "", err
				}
			}
		}
	}
	return rootA, rootB, nil
}

func needsRewrite(path string, size int64) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return info.Size() != size
}

func copyMTime(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}
