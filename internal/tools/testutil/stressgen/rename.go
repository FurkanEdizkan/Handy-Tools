package stressgen

import (
	"fmt"
	"os"
	"path/filepath"
)

// RenameSet writes `count` tiny placeholder files named with a numeric stem
// into dir (e.g. 00001, 00002, …). Used by the rename bench to drive the
// regex `^(\d+)$` -> `file_$1.txt` workload, where the cost is in syscalls,
// not file content.
func RenameSet(dir string, count int) ([]string, error) {
	if err := MkdirAll(dir); err != nil {
		return nil, err
	}
	paths := make([]string, count)
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("%05d", i+1))
		paths[i] = path
		if _, err := os.Stat(path); err == nil {
			continue
		}
		f, err := os.Create(path) //nolint:gosec
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte{'x'}); err != nil {
			f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}
	return paths, nil
}
