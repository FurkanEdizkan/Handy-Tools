//go:build !unix

package installer

import "os/exec"

// detachFromTTY is a no-op off unix; install.sh only runs under a POSIX sh.
func detachFromTTY(*exec.Cmd) {}
