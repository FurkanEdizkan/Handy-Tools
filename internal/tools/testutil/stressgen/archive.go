package stressgen

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
)

// ArchiveTree writes a directory tree with `files` regular files of
// avgSize bytes each, spread across subdirectories with branchFactor files
// per directory. Returns the tree root path (== dir).
//
// File contents are deterministic but not identical — every file gets a
// distinct prefix so a real compressor exercises both dedup and entropy
// codepaths.
func ArchiveTree(dir string, files, avgSize, branchFactor int, seed uint64) (string, error) {
	if err := MkdirAll(dir); err != nil {
		return "", err
	}
	if int64(files)*int64(avgSize) > 1<<20 {
		if err := EnsureFreeSpace(dir); err != nil {
			return "", err
		}
	}
	for i := 0; i < files; i++ {
		sub := i / branchFactor
		subDir := filepath.Join(dir, fmt.Sprintf("d%04d", sub))
		if i%branchFactor == 0 {
			if err := MkdirAll(subDir); err != nil {
				return "", err
			}
		}
		path := filepath.Join(subDir, fmt.Sprintf("f%05d.bin", i))
		if info, err := os.Stat(path); err == nil && info.Size() == int64(avgSize) {
			continue
		}
		r := rand.New(rand.NewPCG(seed, uint64(i))) //nolint:gosec // deterministic fixture
		if err := writeRandomFile(path, int64(avgSize), r); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// ArchiveLarge writes a single regular file of the requested size into dir.
// The file is suitable as the sole entry in a large tar/zip — it exercises
// the per-byte streaming path rather than per-entry overhead.
func ArchiveLarge(dir string, size int64, seed uint64) (string, error) {
	return HashLarge(dir, size, seed)
}
