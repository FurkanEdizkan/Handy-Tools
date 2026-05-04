// Package tools defines the shared progress / error types used by every
// concrete tool implementation under internal/tools/<feature>.
//
// The TUI and the gRPC server both consume these types and translate them
// to/from the proto messages in api/proto/v1.
package tools

import "time"

// Severity mirrors handy.v1.Severity.
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
	CodeAborted          = "ABORTED"
)
