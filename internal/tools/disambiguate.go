package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxUniquePathAttempts caps the suffix counter shared by ReserveUniquePath
// and NextUniquePath. Past 9999, callers see a clean error rather than
// blocking forever or silently overwriting.
const maxUniquePathAttempts = 9999

// compoundExts are double extensions that should be treated as one suffix
// when disambiguating. Without this list, "archive.tar.gz" would become
// "archive.tar-1.gz" instead of "archive-1.tar.gz". Matches the set the web
// frontend recognises in run.ts so behaviour is identical on both sides.
var compoundExts = []string{".tar.gz", ".tar.bz2", ".tar.zst", ".tar.xz"}

// SplitOutputExt splits a path into its stem and extension, recognising the
// compound archive extensions. For "foo/bar.tar.gz" it returns ("foo/bar",
// ".tar.gz"); for plain "foo.png" it returns ("foo", ".png"); for "noext" it
// returns ("noext", "").
func SplitOutputExt(path string) (stem, ext string) {
	lower := strings.ToLower(path)
	for _, c := range compoundExts {
		if strings.HasSuffix(lower, c) {
			return path[:len(path)-len(c)], path[len(path)-len(c):]
		}
	}
	ext = filepath.Ext(path)
	return strings.TrimSuffix(path, ext), ext
}

// ReserveUniquePath returns a path that does not collide with an existing
// file at the moment this function returns. It tries the requested path
// first, then stem-1.ext, stem-2.ext, ... up to maxUniquePathAttempts.
//
// The check uses os.OpenFile with O_CREATE|O_EXCL so the result is atomic
// against other concurrent callers — a stat-then-create loop would race
// under parallelism (two workers can both see "free" and both write). The
// reserved file is closed immediately after creation; the caller is expected
// to reopen it with O_TRUNC (which is what os.Create does anyway).
//
// Use this when multiple goroutines may pick the same target name at once,
// e.g. image-convert's batch worker pool. For sequential pipelines that
// finalise via os.Rename, NextUniquePath is the cheaper choice.
//
// On overflow (every candidate up to the cap exists) returns an error so
// the caller surfaces a clean failure instead of silently overwriting.
func ReserveUniquePath(path string) (string, error) {
	stem, ext := SplitOutputExt(path)
	for i := 0; i <= maxUniquePathAttempts; i++ {
		candidate := path
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d%s", stem, i, ext)
		}
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec
		if err == nil {
			_ = f.Close()
			return candidate, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find a free output path: %s, %s-1..%s-%d all exist",
		path, stem, stem, maxUniquePathAttempts)
}

// NextUniquePath returns a path that does not currently exist, without
// touching the filesystem. It stat-loops the requested path then
// stem-1.ext, stem-2.ext, ... up to maxUniquePathAttempts.
//
// Use this when the actual file creation happens later via an atomic
// os.Rename — e.g. archive.Compress writes <output>.partial then renames.
// In that pipeline there are no concurrent producers, so the lighter
// stat-only probe is sufficient and avoids leaving placeholder zero-byte
// files next to .partial staging files.
//
// For concurrent-writer scenarios (worker pools) use ReserveUniquePath.
func NextUniquePath(path string) (string, error) {
	stem, ext := SplitOutputExt(path)
	for i := 0; i <= maxUniquePathAttempts; i++ {
		candidate := path
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d%s", stem, i, ext)
		}
		_, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find a free output path: %s, %s-1..%s-%d all exist",
		path, stem, stem, maxUniquePathAttempts)
}
