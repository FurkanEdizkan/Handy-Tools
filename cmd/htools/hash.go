package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/hash"
)

// cmdHash handles `htools hash <files>... --algo <name> [--check MANIFEST]`.
// Two modes:
//
//   - Compute (default): hash each positional source with --algo and print
//     `<digest>  <path>` lines in canonical sha256sum format. `--json`
//     swaps that for one JSON object per file.
//   - Verify (--check MANIFEST): recompute each manifest entry's digest
//     and report OK / FAILED. Exit non-zero if any row mismatched.
func cmdHash(ctx context.Context, _ config.Config, args []string) int {
	fs := flag.NewFlagSet("hash", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	algoStr := fs.String("algo", "sha256", "digest algorithm: md5, sha256, blake3")
	check := fs.String("check", "", "path to a sha256sum-format manifest; verify mode")
	strict := fs.Bool("strict", false, "abort before hashing when preflight reports missing/unreadable sources")
	dryRun := fs.Bool("dry-run", false, "report preflight issues for the source list and exit; do not hash anything")
	quiet := fs.Bool("quiet", false, "suppress the trailing summary on stderr")
	asJSON := fs.Bool("json", false, "emit one JSON object per file / verify-entry")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return flagErr(fs, err)
	}
	algo, ok := hash.ParseAlgo(*algoStr)
	if !ok {
		return usageErr(os.Stderr, "hash", fmt.Sprintf("unknown --algo %q (want md5, sha256, blake3)", *algoStr))
	}

	if *check != "" {
		if len(positional) != 0 {
			return usageErr(os.Stderr, "hash", "--check is exclusive with positional sources")
		}
		return runVerify(ctx, *check, algo, *quiet, *asJSON)
	}
	if len(positional) == 0 {
		return usageErr(os.Stderr, "hash", "need at least one source (or --check MANIFEST)")
	}
	opts := progressOpts{quiet: *quiet, json: *asJSON, strict: *strict}
	if *dryRun || *strict {
		ins, terr := hash.Inspect(hash.Request{Sources: positional, Algo: algo})
		if terr != nil {
			fmt.Fprintf(os.Stderr, "hash: %s\n", terr.Error())
			return exitCode(terr)
		}
		if code := runPreflight(ins.Issues, opts); code != 0 {
			return code
		}
		if *dryRun {
			if !*quiet {
				fmt.Fprintf(os.Stderr, "dry-run: %d source(s) ready, %d issue(s)\n", len(positional)-len(ins.Issues), len(ins.Issues))
			}
			return 0
		}
	}
	return runHash(ctx, positional, algo, *quiet, *asJSON)
}

// runHash computes digests for every source path via hash.Run, which fans
// the work out across runtime.GOMAXPROCS(0) goroutines. Prints results in
// canonical `<digest>  <path>` format (two spaces) or as one JSON object
// per file when --json is set, followed in JSON mode by a terminal
// {"completed":true,...} line whose "failures" lists every file that could
// not be hashed. Exits 1 if any file failed (like sha256sum), 2 when the
// run was interrupted, 0 otherwise.
//
// Output order is per-file completion order, not source-list order — the
// parallel worker pool means smaller files print first when batched with
// large ones. The --quiet stderr summary stays identical.
func runHash(ctx context.Context, sources []string, algo hash.Algo, quiet, asJSON bool) int {
	enc := json.NewEncoder(os.Stdout)
	var perFileFailed int
	var completed bool
	for ev := range hash.Run(ctx, hash.Request{Sources: sources, Algo: algo}) {
		if ev.Completed {
			completed = true
			if asJSON {
				_ = enc.Encode(progressJSON{
					Tool: ev.Tool, Action: ev.Action, Completed: true, Message: ev.Message,
					ErrorCode: errCode(ev.Err), ErrorDetail: errDetail(ev.Err),
					Failures: failuresJSON(ev.Failures),
				})
			}
			// Terminal event with a structured Err: surface and exit.
			if ev.Err != nil {
				if !asJSON {
					fmt.Fprintf(os.Stderr, "hash: %s\n", ev.Err.Message)
				}
				return exitCode(ev.Err)
			}
			// Terminal success: print the summary line on stderr.
			if !quiet && !asJSON {
				fmt.Fprintf(os.Stderr, "hash: %s\n", ev.Message)
			}
			continue
		}
		// Per-file failure (SeverityError, not Completed). JSON consumers get
		// it from the terminal line's failures list instead.
		if ev.Level == tools.SeverityError {
			perFileFailed++
			if !asJSON {
				fmt.Fprintf(os.Stderr, "hash: %s\n", ev.Message)
			}
			continue
		}
		// Per-file success: ev.Message already has the `<digest>  <path>` shape.
		if asJSON {
			digest, path, ok := parseHashLine(ev.Message)
			if !ok {
				continue
			}
			_ = enc.Encode(&hash.Result{Path: path, Digest: digest, Algo: algo})
		} else {
			fmt.Println(ev.Message)
		}
	}
	if !completed {
		// Cancelled (Ctrl-C): the terminal event was dropped, so the digests
		// printed so far are not the full set.
		return finalize(tools.Progress{}, progressOpts{quiet: quiet, json: asJSON})
	}
	if perFileFailed > 0 {
		return 1
	}
	return 0
}

// parseHashLine splits a hash.Run success message of the form
// `<digest>  <path>` (two-space separator, matching sha256sum). Used only
// by --json mode to reconstruct a hash.Result for encoding.
func parseHashLine(msg string) (digest, path string, ok bool) {
	i := strings.Index(msg, "  ")
	if i <= 0 || i >= len(msg)-2 {
		return "", "", false
	}
	return msg[:i], msg[i+2:], true
}

// runVerify reads a manifest and reports OK / FAILED per entry. Exits 1 on
// the first mismatch / per-entry error; 0 when every row matched.
func runVerify(ctx context.Context, manifestPath string, algo hash.Algo, quiet, asJSON bool) int {
	// Relative entries resolve like `sha256sum -c` (against the working
	// directory) so `htools hash d/a.txt > d/SUMS; htools hash --check d/SUMS`
	// round-trips; the manifest's own directory is the fallback, so a
	// manifest of bare names still verifies when run from elsewhere.
	dirs := []string{filepath.Dir(manifestPath)}
	if cwd, err := os.Getwd(); err == nil {
		dirs = []string{cwd, filepath.Dir(manifestPath)}
	}
	entries, terr := hash.VerifyWith(ctx, manifestPath, algo, hash.VerifyOptions{SearchDirs: dirs})
	if terr != nil {
		fmt.Fprintf(os.Stderr, "hash --check: %s\n", terr.Message)
		return exitCode(terr)
	}
	enc := json.NewEncoder(os.Stdout)
	var matched, mismatched int
	for _, e := range entries {
		if asJSON {
			_ = enc.Encode(e)
		} else {
			status := "OK"
			if !e.OK {
				status = "FAILED"
				if e.Err != "" {
					status = "FAILED (" + e.Err + ")"
				}
			}
			fmt.Printf("%s: %s\n", e.Path, status)
		}
		if e.OK {
			matched++
		} else {
			mismatched++
		}
	}
	if !quiet {
		fmt.Fprintf(os.Stderr, "hash --check: %d matched, %d mismatched, %d total\n",
			matched, mismatched, len(entries))
	}
	if mismatched > 0 {
		return 1
	}
	return 0
}
