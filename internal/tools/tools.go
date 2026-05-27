// Package tools defines the shared progress / error types used by every
// concrete tool implementation under internal/tools/<feature>.
//
// The TUI and the gRPC server both consume these types and translate them
// to/from the proto messages in api/proto/v1.
package tools

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Severity mirrors handytools.v1.Severity.
type Severity int

const (
	SeverityUnspecified Severity = iota
	SeverityInfo
	SeverityWarning
	SeverityError
)

// Progress is sent on a tool's progress channel for every meaningful step.
// At most one of (BytesDone, BytesTotal) and Fraction is meaningful — tools
// fill in whichever they can measure.
type Progress struct {
	JobID       string
	Tool        string
	Action      string
	StartedAt   time.Time
	CurrentItem string
	BytesDone   int64
	BytesTotal  int64
	Fraction    float64
	Level       Severity
	Message     string
	Completed   bool
	Err         *Error
}

// Error is a structured failure reported by a tool.
type Error struct {
	Code    string
	Message string
	Detail  string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail != "" {
		return e.Code + ": " + e.Message + " (" + e.Detail + ")"
	}
	return e.Code + ": " + e.Message
}

// Common error codes used across tool packages.
const (
	CodeMissingBinary    = "MISSING_BINARY"
	CodeUnsupportedInput = "UNSUPPORTED_INPUT"
	CodeBadRequest       = "BAD_REQUEST"
	CodeIO               = "IO_ERROR"
	CodePermissionDenied = "PERMISSION_DENIED"
	CodeNotFound         = "NOT_FOUND"
	CodeAborted          = "ABORTED"
)

// PathIssue is a preflight-detected problem with a single input or output
// path. Reported by tool Inspect() functions so callers can warn / abort
// before any destructive operation. Code is one of the CodeXxx constants —
// typically CodePermissionDenied or CodeNotFound for filesystem problems.
type PathIssue struct {
	Path   string
	Code   string
	Detail string
}

// StatInputs returns a PathIssue for each path that fails os.Stat. Paths that
// stat cleanly are omitted. Result is in input order so callers can correlate
// with their source list. Returns nil (not an empty slice) when every path
// stats cleanly so callers can use `len(issues) > 0` and `issues == nil`
// interchangeably.
func StatInputs(paths []string) []PathIssue {
	var issues []PathIssue
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			issues = append(issues, PathIssue{
				Path:   p,
				Code:   ClassifyFSError(err),
				Detail: err.Error(),
			})
		}
	}
	return issues
}

// CheckOutputDirWritable returns a PathIssue when dir doesn't exist or the
// current process can't create a file inside it. Returns nil when dir is
// writable. The probe is a short-lived O_CREATE|O_EXCL temp file that's
// removed immediately on success — this catches read-only mounts and
// directory permission bits that os.Stat alone wouldn't surface.
func CheckOutputDirWritable(dir string) *PathIssue {
	info, err := os.Stat(dir)
	if err != nil {
		return &PathIssue{Path: dir, Code: ClassifyFSError(err), Detail: err.Error()}
	}
	if !info.IsDir() {
		return &PathIssue{Path: dir, Code: CodeBadRequest, Detail: "not a directory"}
	}
	probe := filepath.Join(dir, fmt.Sprintf(".handy-write-probe-%d-%d", os.Getpid(), time.Now().UnixNano()))
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return &PathIssue{Path: dir, Code: ClassifyFSError(err), Detail: err.Error()}
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return nil
}

// ClassifyFSError returns the most specific tools code for a filesystem error.
// Returns "" on nil so callers can short-circuit. Falls back to CodeIO so it
// can be used as a one-liner classifier at every os.Open / os.Stat / os.Rename
// error-wrap site.
func ClassifyFSError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, fs.ErrPermission):
		return CodePermissionDenied
	case errors.Is(err, fs.ErrNotExist):
		return CodeNotFound
	default:
		return CodeIO
	}
}
