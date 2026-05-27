package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools/rename"
)

// cmdRename handles `htools rename <dir> --pattern <re> --replace <tmpl> ...`.
// One positional argument: the directory to walk. Files in that dir whose
// basename matches --pattern are renamed using --replace (Go regexp template
// with $1..$9 from capture groups).
func cmdRename(ctx context.Context, _ config.Config, args []string) int {
	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pattern := fs.String("pattern", "", "regex matched against each file's basename (required)")
	replace := fs.String("replace", "", "Go regexp replacement template; supports $1..$9 (required)")
	recurse := fs.Bool("recurse", false, "descend into subdirectories")
	collision := fs.String("on-collision", "error", "what to do when the target exists: error|skip|suffix")
	dryRun := fs.Bool("dry-run", false, "print the rename plan and exit; do not move anything")
	strict := fs.Bool("strict", false, "abort before any rename when preflight reports missing/unreadable sources or unwritable targets")
	rollback := fs.Bool("rollback-on-error", false, "abort on first rename failure and undo every rename already done in this batch")
	quiet := fs.Bool("quiet", false, "suppress per-event progress lines")
	asJSON := fs.Bool("json", false, "emit one JSON object per progress event")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, "rename", err.Error())
	}
	if len(positional) != 1 {
		return usageErr(os.Stderr, "rename", "need exactly one directory")
	}
	if *pattern == "" {
		return usageErr(os.Stderr, "rename", "--pattern is required")
	}
	if *replace == "" {
		return usageErr(os.Stderr, "rename", "--replace is required (use --replace '' if you really mean empty)")
	}
	mode, ok := parseCollision(*collision)
	if !ok {
		return usageErr(os.Stderr, "rename", fmt.Sprintf("unknown --on-collision %q (want error, skip, suffix)", *collision))
	}

	sources, err := listFiles(positional[0], *recurse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rename: list %s: %v\n", positional[0], err)
		return 1
	}
	if len(sources) == 0 {
		fmt.Fprintf(os.Stderr, "rename: no files under %s\n", positional[0])
		return 1
	}

	req := rename.Request{
		Sources:     sources,
		Pattern:     *pattern,
		Replace:     *replace,
		OnCollision: mode,
		Rollback:    *rollback,
	}

	opts := progressOpts{quiet: *quiet, json: *asJSON, strict: *strict}
	if *dryRun {
		return runRenameDryRun(req, opts)
	}
	if *strict {
		ins, terr := rename.Inspect(req)
		if terr != nil {
			fmt.Fprintf(os.Stderr, "rename: %s\n", terr.Error())
			return exitCode(terr)
		}
		if code := runPreflight(ins.Issues, opts); code != 0 {
			return code
		}
	}
	ch := rename.Run(ctx, req)
	return streamProgress(ch, opts)
}

// runRenameDryRun prints the rename plan as `from -> to` lines (or as one
// JSON object per row when --json is set later — for now plain text).
// Returns 0 on a successful inspection regardless of whether any renames
// would happen — the caller can grep the output to decide. When --strict is
// set, preflight issues abort with exit 2 before any plan is printed.
func runRenameDryRun(req rename.Request, opts progressOpts) int {
	ins, terr := rename.Inspect(req)
	if terr != nil {
		fmt.Fprintf(os.Stderr, "rename: %s\n", terr.Error())
		return exitCode(terr)
	}
	if code := runPreflight(ins.Issues, opts); code != 0 {
		return code
	}
	var moved int
	for _, p := range ins.Plans {
		if p.From == p.To {
			fmt.Fprintf(os.Stdout, "skip  %s\n", p.From)
			continue
		}
		fmt.Fprintf(os.Stdout, "move  %s -> %s\n", p.From, p.To)
		moved++
	}
	fmt.Fprintf(os.Stderr, "dry-run: %d planned, %d skipped\n", moved, len(ins.Plans)-moved)
	return 0
}

// parseCollision maps the --on-collision flag value to a rename.Collision.
func parseCollision(s string) (rename.Collision, bool) {
	switch strings.ToLower(s) {
	case "error", "":
		return rename.CollisionError, true
	case "skip":
		return rename.CollisionSkip, true
	case "suffix":
		return rename.CollisionSuffix, true
	}
	return rename.CollisionError, false
}

// listFiles enumerates regular files under dir. With recurse=false only the
// top-level entries are returned; with recurse=true a filepath.WalkDir
// descends into subdirectories. Directories are never returned as source
// paths (rename operates on files only).
func listFiles(dir string, recurse bool) ([]string, error) {
	info, err := os.Stat(dir) //nolint:gosec // dir comes from the user's CLI invocation
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	var out []string
	if !recurse {
		entries, err := os.ReadDir(dir) //nolint:gosec // dir comes from the user's CLI invocation
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			out = append(out, filepath.Join(dir, e.Name()))
		}
		return out, nil
	}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error { //nolint:gosec // dir comes from the user's CLI invocation
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		out = append(out, path)
		return nil
	})
	return out, err
}
