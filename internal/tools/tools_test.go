package tools

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyFSError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"plain", errors.New("boom"), CodeIO},
		{"ErrPermission", fs.ErrPermission, CodePermissionDenied},
		{"ErrNotExist", fs.ErrNotExist, CodeNotFound},
		{"wrapped ErrPermission", fmt.Errorf("open foo: %w", fs.ErrPermission), CodePermissionDenied},
		{"wrapped ErrNotExist", fmt.Errorf("stat foo: %w", fs.ErrNotExist), CodeNotFound},
		{"double-wrapped ErrPermission", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", fs.ErrPermission)), CodePermissionDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyFSError(tc.err)
			if got != tc.want {
				t.Fatalf("ClassifyFSError(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestClassifyFSError_RealOSError exercises the helper against an os.PathError
// produced by a real syscall (chmod-000 file). This catches accidental
// regressions where the helper would only work against synthetic fs.Err* but
// not the wrapped *PathError shape that os.* returns.
func TestClassifyFSError_RealOSError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if os.Geteuid() == 0 {
		t.Skip("running as root — file modes are bypassed, can't observe EACCES")
	}

	_, err := os.Open(path)
	if err == nil {
		t.Skip("open succeeded despite chmod 000 — probably running on a filesystem that ignores modes")
	}
	if got := ClassifyFSError(err); got != CodePermissionDenied {
		t.Fatalf("ClassifyFSError(real EACCES) = %q, want %q (err=%v)", got, CodePermissionDenied, err)
	}

	missing := filepath.Join(dir, "does-not-exist")
	_, err = os.Open(missing)
	if err == nil {
		t.Fatal("expected open of missing file to fail")
	}
	if got := ClassifyFSError(err); got != CodeNotFound {
		t.Fatalf("ClassifyFSError(real ENOENT) = %q, want %q (err=%v)", got, CodeNotFound, err)
	}
}
