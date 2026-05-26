package stressgen

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
)

// HashSet writes count random files of size bytes each into dir. Returns the
// absolute paths of the generated files in deterministic order. Reuses
// existing files when they already match the spec — reruns are cheap.
func HashSet(dir string, count int, size int64, seed uint64) ([]string, error) {
	if err := MkdirAll(dir); err != nil {
		return nil, err
	}
	if int64(count)*size > 1<<20 {
		if err := EnsureFreeSpace(dir); err != nil {
			return nil, err
		}
	}
	paths := make([]string, count)
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file_%05d.bin", i))
		paths[i] = path
		if info, err := os.Stat(path); err == nil && info.Size() == size {
			continue
		}
		// Distinct per-file PRNG so an early failure doesn't desync later files.
		r := rand.New(rand.NewPCG(seed, uint64(i))) //nolint:gosec // deterministic stress fixture, not crypto
		if err := writeRandomFile(path, size, r); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// HashLarge writes a single file of size bytes filled with deterministic
// pseudo-random bytes. Used for the 1 GiB single-file path through hash.Run.
func HashLarge(dir string, size int64, seed uint64) (string, error) {
	if err := MkdirAll(dir); err != nil {
		return "", err
	}
	if size > 1<<20 {
		if err := EnsureFreeSpace(dir); err != nil {
			return "", err
		}
	}
	path := filepath.Join(dir, fmt.Sprintf("large_%d.bin", size))
	if info, err := os.Stat(path); err == nil && info.Size() == size {
		return path, nil
	}
	r := rand.New(rand.NewPCG(seed, 0xfeed)) //nolint:gosec // deterministic stress fixture
	if err := writeRandomFile(path, size, r); err != nil {
		return "", err
	}
	return path, nil
}

// writeRandomFile streams size bytes of PRNG-derived data into path. Uses a
// 1 MiB buffer filled in 8-byte chunks (binary.LittleEndian) so the rand
// source's per-call cost amortises across 8 bytes of output.
func writeRandomFile(path string, size int64, r *rand.Rand) error {
	f, err := os.Create(path) //nolint:gosec // deterministic stress fixture
	if err != nil {
		return err
	}
	const bufSize = 1 << 20
	buf := make([]byte, bufSize)
	var written int64
	for written < size {
		for i := 0; i+8 <= bufSize; i += 8 {
			binary.LittleEndian.PutUint64(buf[i:i+8], r.Uint64())
		}
		remaining := size - written
		write := int64(bufSize)
		if remaining < write {
			write = remaining
		}
		n, werr := f.Write(buf[:write])
		if werr != nil {
			f.Close()
			os.Remove(path)
			return werr
		}
		written += int64(n)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}
