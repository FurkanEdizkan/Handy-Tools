// Package server hosts the gRPC service implementations. Every handler is a
// thin adapter over a corresponding internal/tools/<feature> package: it maps
// proto requests onto the tool's plain-Go API and forwards progress channel
// events onto the streaming response.
package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// Options control common server behavior. AllowRoots is a list of filesystem
// roots that incoming FileRef paths must reside under. An empty AllowRoots
// fails closed — every path is rejected. Callers compose Options once at
// startup; handlers consult it on every request.
type Options struct {
	AllowRoots []string
}

// CheckPath verifies that p resolves to an absolute path inside one of the
// configured allow-roots. It returns the cleaned absolute path or an error.
//
// A rejection is always a *tools.Error with CodeBadRequest: the caller asked
// for a path the server's policy forbids, which is a request problem, not a
// filesystem one (PERMISSION_DENIED is reserved for real EACCES, which the
// user fixes with chmod rather than by changing the request). Returning a
// typed error keeps the code identical on every transport — sync and async
// HTTP, gRPC, and MCP.
func (o Options) CheckPath(p string) (string, error) {
	if p == "" {
		return "", &tools.Error{Code: tools.CodeBadRequest, Message: "empty path"}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	roots := o.AllowRoots
	if len(roots) == 0 {
		// No roots configured -> reject everything to fail closed.
		return "", outsideRoots(abs)
	}
	for _, r := range roots {
		rabs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		rabs = filepath.Clean(rabs)
		// Append a separator so "/foo" doesn't match "/foobar", except when
		// rabs is already exactly the separator (root "/"); doubling it would
		// produce "//", which no real path starts with — so AllowRoots:["/"]
		// would silently reject everything.
		if !strings.HasSuffix(rabs, string(filepath.Separator)) {
			rabs += string(filepath.Separator)
		}
		if strings.HasPrefix(abs+string(filepath.Separator), rabs) {
			return abs, nil
		}
	}
	return "", outsideRoots(abs)
}

// outsideRoots is the CheckPath rejection for a path no allow-root covers.
func outsideRoots(abs string) *tools.Error {
	return &tools.Error{
		Code:    tools.CodeBadRequest,
		Message: fmt.Sprintf("path %q not under any allowed root", abs),
	}
}

// ToolError normalizes any handler error into a *tools.Error so every
// transport reports the same code for the same failure. A *tools.Error
// (possibly wrapped) passes through unchanged. A plain error that wraps a
// filesystem or context error is classified via tools.ClassifyFSError
// (NOT_FOUND, PERMISSION_DENIED, ABORTED, ...), and any other error the OS
// returned (EIO, ENOSPC, ...) stays IO_ERROR — that's a server-side failure,
// not the caller's. Anything else — typically a malformed request or
// unparseable input — is BAD_REQUEST. Returns nil for nil.
func ToolError(err error) *tools.Error {
	if err == nil {
		return nil
	}
	var te *tools.Error
	if errors.As(err, &te) {
		return te
	}
	code := tools.ClassifyFSError(err)
	if code == tools.CodeIO && !isOSError(err) {
		code = tools.CodeBadRequest
	}
	return &tools.Error{Code: code, Message: err.Error()}
}

// isOSError reports whether err came back from a system call (a failed open,
// read, rename, ...).
func isOSError(err error) bool {
	var (
		pathErr *fs.PathError
		linkErr *os.LinkError
		sysErr  *os.SyscallError
		errno   syscall.Errno
	)
	return errors.As(err, &pathErr) || errors.As(err, &linkErr) ||
		errors.As(err, &sysErr) || errors.As(err, &errno)
}

// forwardProgress drains a tool's progress channel into emit. Tool packages
// send through a select on ctx.Done(), so once ctx is canceled (a job
// timeout, an MCP cancellation) the terminal event can be dropped and the
// channel simply closes. If that happens — no Completed event was forwarded
// and ctx is done — forwardProgress returns an ABORTED error so the caller
// records a terminal failure instead of silently reporting success.
func forwardProgress(ctx context.Context, ch <-chan tools.Progress, emit func(tools.Progress) error) error {
	sawTerminal := false
	for prog := range ch {
		if prog.Completed {
			sawTerminal = true
		}
		if err := emit(prog); err != nil {
			return err
		}
	}
	if !sawTerminal {
		if err := ctx.Err(); err != nil {
			return &tools.Error{Code: tools.CodeAborted, Message: "operation canceled", Detail: err.Error()}
		}
	}
	return nil
}

// asProto converts a tool Progress event into the proto wire form. It is
// declared here so handlers don't reach into internal/tools and the proto
// package directly. The actual proto types live in gen/handytools/v1; the
// `make proto` (`buf generate`) target populates that package, so this
// helper is implemented in adapters_proto.go alongside the generated code.

// errorFromTool turns a tool error into a wire-friendly tuple.
func errorFromTool(e *tools.Error) (code, msg, detail string) {
	if e == nil {
		return "", "", ""
	}
	return e.Code, e.Message, e.Detail
}
