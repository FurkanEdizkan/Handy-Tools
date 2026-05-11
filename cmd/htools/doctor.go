package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// runDoctor prints which optional system tools are present and which features
// they unlock. Designed to be readable on a plain terminal (no TUI).
func runDoctor() {
	writeDoctor(os.Stdout)
}

// writeDoctor renders the doctor report to w. Split out so tests can capture
// the output deterministically.
func writeDoctor(w io.Writer) {
	fmt.Fprintln(w, "Handy Tools environment check")
	fmt.Fprintln(w, strings.Repeat("-", 32))
	any := false
	for _, r := range sysdep.All() {
		any = true
		mark := "[ ]"
		if r.Found {
			mark = "[x]"
		}
		fmt.Fprintf(w, "%s %-12s %s\n", mark, r.Tool.Name, r.Tool.Description)
		for _, feat := range r.Tool.Features {
			fmt.Fprintf(w, "        %s %s\n", marker(r.Found), feat)
		}
		if !r.Found {
			if hint := r.Tool.InstallHint[runtime.GOOS]; hint != "" {
				fmt.Fprintf(w, "        install: %s\n", hint)
			}
		}
		fmt.Fprintln(w)
	}
	if !any {
		fmt.Fprintln(w, "(no optional tools registered)")
	}
}

func marker(found bool) string {
	if found {
		return "+"
	}
	return "-"
}
