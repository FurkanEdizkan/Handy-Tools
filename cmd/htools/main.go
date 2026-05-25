// Command htools is the non-interactive CLI for Handy Tools. It dispatches
// each invocation to one of the tool subcommands (convert, pack, extract,
// inspect, pdf, rename, hash, doctor) using stdlib flag — one run, one
// operation, no terminal UI.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/furkandedizkan/handy-tools/internal/buildinfo"
	"github.com/furkandedizkan/handy-tools/internal/config"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is the testable entry point — it returns the exit code instead of
// calling os.Exit so unit tests can drive the dispatch directly.
func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}
	cfg, _, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		cfg = config.Defaults()
	}
	return dispatch(context.Background(), cfg, args[0], args[1:])
}

func dispatch(ctx context.Context, cfg config.Config, verb string, args []string) int {
	switch verb {
	case "convert":
		return cmdConvert(ctx, cfg, args)
	case "pack":
		return cmdPack(ctx, cfg, args)
	case "extract":
		return cmdExtract(ctx, cfg, args)
	case "inspect":
		return cmdInspect(ctx, cfg, args)
	case "pdf":
		return cmdPDF(ctx, cfg, args)
	case "rename":
		return cmdRename(ctx, cfg, args)
	case "hash":
		return cmdHash(ctx, cfg, args)
	case "diff-tree":
		return cmdDiffTree(ctx, cfg, args)
	case "doctor":
		return cmdDoctor()
	case "version", "--version", "-v":
		fmt.Println(buildinfo.String())
		return 0
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return 0
	}
	fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", verb)
	printUsage(os.Stderr)
	return 2
}
