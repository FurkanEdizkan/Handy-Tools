package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools/difftree"
)

// cmdDiffTree handles `htools diff-tree <a> <b> [--by mtime|hash] ...`.
//
// Exit code semantics mirror GNU diff so this composes with shell scripts:
//
//	0  trees match (no diffs found)
//	1  trees differ
//	2  usage / I/O error
//
// The walker doesn't follow symlinks, so a symlink-loop fixture cannot hang
// the tool — that's enforced at the package level in internal/tools/difftree.
func cmdDiffTree(_ context.Context, _ config.Config, args []string) int {
	fs := flag.NewFlagSet("diff-tree", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	by := fs.String("by", "mtime", "compare strategy: mtime (size + mtime) | hash (SHA256 content)")
	quiet := fs.Bool("quiet", false, "suppress the trailing summary on stderr")
	asJSON := fs.Bool("json", false, "emit one JSON object per diff entry and a final summary object")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, "diff-tree", err.Error())
	}
	if len(positional) != 2 {
		return usageErr(os.Stderr, "diff-tree", "need exactly two paths: <a> <b>")
	}
	mode, ok := difftree.ParseMode(*by)
	if !ok {
		return usageErr(os.Stderr, "diff-tree", fmt.Sprintf("unknown --by %q (want mtime or hash)", *by))
	}

	diffs, terr := difftree.Inspect(difftree.Request{
		A:    positional[0],
		B:    positional[1],
		Mode: mode,
	})
	if terr != nil {
		if *asJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"error":  terr.Code,
				"detail": terr.Message,
			})
		} else {
			fmt.Fprintf(os.Stderr, "diff-tree: %s\n", terr.Error())
		}
		return exitCode(terr)
	}

	var added, removed, changed int
	enc := json.NewEncoder(os.Stdout)
	for _, d := range diffs {
		switch d.Status {
		case difftree.StatusAdded:
			added++
		case difftree.StatusRemoved:
			removed++
		case difftree.StatusChanged:
			changed++
		}
		if *asJSON {
			_ = enc.Encode(d)
			continue
		}
		switch d.Status {
		case difftree.StatusChanged:
			fmt.Printf("changed %s (%s)\n", d.Path, d.Reason)
		default:
			fmt.Printf("%s %s\n", d.Status, d.Path)
		}
	}

	if !*quiet {
		fmt.Fprintf(os.Stderr, "diff-tree: %d added, %d removed, %d changed (mode=%s)\n",
			added, removed, changed, mode)
	}
	if len(diffs) > 0 {
		return 1
	}
	return 0
}
