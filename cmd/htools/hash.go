package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/furkandedizkan/handy-tools/internal/config"
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
	quiet := fs.Bool("quiet", false, "suppress the trailing summary on stderr")
	asJSON := fs.Bool("json", false, "emit one JSON object per file / verify-entry")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, "hash", err.Error())
	}
	algo, ok := hash.ParseAlgo(*algoStr)
	if !ok {
		return usageErr(os.Stderr, "hash", fmt.Sprintf("unknown --algo %q (want md5, sha256, blake3)", *algoStr))
	}

	if *check != "" {
		if len(positional) != 0 {
			return usageErr(os.Stderr, "hash", "--check is exclusive with positional sources")
		}
		return runVerify(*check, algo, *quiet, *asJSON)
	}
	if len(positional) == 0 {
		return usageErr(os.Stderr, "hash", "need at least one source (or --check MANIFEST)")
	}
	return runHash(positional, algo, *quiet, *asJSON)
}

// runHash computes digests for every source path. Prints results in
// canonical `<digest>  <path>` format (two spaces) or as one JSON object
// per file when --json is set. Exits 1 if any file failed; 0 otherwise.
func runHash(sources []string, algo hash.Algo, quiet, asJSON bool) int {
	enc := json.NewEncoder(os.Stdout)
	var failed int
	for _, src := range sources {
		res, terr := hash.Hash(src, algo)
		if terr != nil {
			failed++
			fmt.Fprintf(os.Stderr, "hash: %s: %s\n", src, terr.Message)
			continue
		}
		if asJSON {
			_ = enc.Encode(res)
		} else {
			fmt.Printf("%s  %s\n", res.Digest, res.Path)
		}
	}
	if !quiet {
		total := len(sources)
		fmt.Fprintf(os.Stderr, "hash: %d/%d file(s) hashed with %s\n", total-failed, total, algo)
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// runVerify reads a manifest and reports OK / FAILED per entry. Exits 1 on
// the first mismatch / per-entry error; 0 when every row matched.
func runVerify(manifestPath string, algo hash.Algo, quiet, asJSON bool) int {
	entries, terr := hash.Verify(manifestPath, algo)
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
