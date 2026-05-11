// Package server hosts the gRPC service implementations. Every handler is a
// thin adapter over a corresponding internal/tools/<feature> package: it maps
// proto requests onto the tool's plain-Go API and forwards progress channel
// events onto the streaming response.
package server

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// Options control common server behavior. AllowRoots is a list of filesystem
// roots that incoming FileRef paths must reside under. An empty AllowRoots
// means the server only serves files under its current working directory at
// startup time. Callers compose Options once at startup; handlers consult it
// on every request.
type Options struct {
	AllowRoots []string
}

// CheckPath verifies that p resolves to an absolute path inside one of the
// configured allow-roots. It returns the cleaned absolute path or an error.
func (o Options) CheckPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	roots := o.AllowRoots
	if len(roots) == 0 {
		// No roots configured -> reject everything to fail closed.
		return "", fmt.Errorf("path %q not under any allowed root", abs)
	}
	for _, r := range roots {
		rabs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		rabs = filepath.Clean(rabs) + string(filepath.Separator)
		if strings.HasPrefix(abs+string(filepath.Separator), rabs) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("path %q not under any allowed root", abs)
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
