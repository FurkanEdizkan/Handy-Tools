package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/furkandedizkan/handy/internal/tools/sysdep"
)

// runDoctor prints which optional system tools are present and which features
// they unlock. Designed to be readable on a plain terminal (no TUI).
func runDoctor() {
	fmt.Println("Handy environment check")
	fmt.Println(strings.Repeat("-", 32))
	any := false
	for _, r := range sysdep.All() {
		any = true
		mark := "[ ]"
		if r.Found {
			mark = "[x]"
		}
		fmt.Printf("%s %-12s %s\n", mark, r.Tool.Name, r.Tool.Description)
		for _, feat := range r.Tool.Features {
			fmt.Printf("        %s %s\n", marker(r.Found), feat)
		}
		if !r.Found {
			if hint := r.Tool.InstallHint[runtime.GOOS]; hint != "" {
				fmt.Printf("        install: %s\n", hint)
			}
		}
		fmt.Println()
	}
	if !any {
		fmt.Println("(no optional tools registered)")
	}
}

func marker(found bool) string {
	if found {
		return "+"
	}
	return "-"
}
