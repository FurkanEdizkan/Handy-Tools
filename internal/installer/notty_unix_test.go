//go:build unix

package installer

import (
	"os/exec"
	"syscall"
)

// detachFromTTY runs cmd in a new session with no controlling terminal, so the
// installer's `read </dev/tty` confirmation can't block on the terminal of the
// developer running `go test`.
func detachFromTTY(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
