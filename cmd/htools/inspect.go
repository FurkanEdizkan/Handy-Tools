package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/klauspost/compress/zstd"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
)

// cmdInspect handles `htools inspect <archive> [--grep <pat>] ...`. Without
// --grep it prints the archive.Inspection metadata (format, multi-part flag,
// entry count, uncompressed size). With --grep it streams content matches as
// `<path>:<line-num>:<text>` lines without extracting the archive.
func cmdInspect(ctx context.Context, _ config.Config, args []string) int {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pattern := fs.String("grep", "", "regex to search inside the archive's text entries; without this flag, only metadata is printed")
	maxBytes := fs.Int64("max-bytes", 1<<20, "skip --grep entries larger than this (default 1 MiB)")
	quiet := fs.Bool("quiet", false, "suppress per-entry status (only matches and final summary)")
	asJSON := fs.Bool("json", false, "emit metadata or matches as JSON")
	srcs, err := parseFlags(fs, args)
	if err != nil {
		return usageErr(os.Stderr, "inspect", err.Error())
	}
	if len(srcs) != 1 {
		return usageErr(os.Stderr, "inspect", "need exactly one archive path")
	}
	source := srcs[0]

	ins, err := archive.Inspect(ctx, source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inspect: %v\n", err)
		return 1
	}

	if *pattern == "" {
		return printInspection(source, ins, *asJSON)
	}
	re, rerr := regexp.Compile(*pattern)
	if rerr != nil {
		return usageErr(os.Stderr, "inspect", fmt.Sprintf("invalid --grep regex: %v", rerr))
	}
	return grepArchive(source, ins, re, *maxBytes, *quiet, *asJSON)
}

// printInspection prints the metadata returned by archive.Inspect. JSON mode
// emits a single object; plain mode prints a readable label-value layout.
func printInspection(source string, ins *archive.Inspection, asJSON bool) int {
	if asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"source":           source,
			"format":           ins.Format.String(),
			"multi_part":       ins.MultiPart,
			"detected_parts":   ins.DetectedParts,
			"missing_parts":    ins.MissingParts,
			"entry_count":      ins.EntryCount,
			"uncompressed_sz":  ins.UncompressedSz,
			"requires_pwd":     ins.RequiresPwd,
			"requires_binary":  ins.RequiresBinary,
			"binary_available": ins.BinaryAvailable,
		})
		return 0
	}
	fmt.Printf("%-18s %s\n", "Source:", source)
	fmt.Printf("%-18s %s\n", "Format:", ins.Format.String())
	if ins.MultiPart {
		fmt.Printf("%-18s yes (%d parts found", "Multi-part:", len(ins.DetectedParts))
		if n := len(ins.MissingParts); n > 0 {
			fmt.Printf(", %d missing", n)
		}
		fmt.Println(")")
	} else {
		fmt.Printf("%-18s no\n", "Multi-part:")
	}
	if ins.EntryCount >= 0 {
		fmt.Printf("%-18s %d\n", "Entries:", ins.EntryCount)
	}
	if ins.UncompressedSz >= 0 {
		fmt.Printf("%-18s %d bytes\n", "Uncompressed:", ins.UncompressedSz)
	}
	if ins.RequiresBinary != "" {
		state := "missing"
		if ins.BinaryAvailable {
			state = "found on PATH"
		}
		fmt.Printf("%-18s %s (%s)\n", "Needs binary:", ins.RequiresBinary, state)
	}
	if ins.RequiresPwd {
		fmt.Printf("%-18s yes\n", "Password:")
	}
	return 0
}

// grepArchive iterates entries in the archive and prints regex matches as
// `<path>:<line>:<text>`. Pure-Go formats only (zip / tar / tar.gz /
// tar.bz2 / tar.zst). RAR and 7z entries emit a single skip-warning since
// the unrar/7z binaries don't expose per-entry stream readers cleanly.
func grepArchive(source string, ins *archive.Inspection, re *regexp.Regexp, maxBytes int64, quiet, asJSON bool) int {
	switch ins.Format {
	case archive.FormatZip:
		return grepZip(source, re, maxBytes, quiet, asJSON)
	case archive.FormatTar:
		return grepTar(source, plainOpener, re, maxBytes, quiet, asJSON)
	case archive.FormatTarGz:
		return grepTar(source, gzipOpener, re, maxBytes, quiet, asJSON)
	case archive.FormatTarBz2:
		return grepTar(source, bzip2Opener, re, maxBytes, quiet, asJSON)
	case archive.FormatTarZst:
		return grepTar(source, zstdOpener, re, maxBytes, quiet, asJSON)
	case archive.FormatRar, archive.FormatSevenZ:
		fmt.Fprintf(os.Stderr, "inspect: --grep skips %s entries (the delegated %s binary doesn't expose per-entry stream readers)\n", ins.Format, ins.RequiresBinary)
		return 1
	default:
		fmt.Fprintf(os.Stderr, "inspect: --grep unsupported for format %s\n", ins.Format)
		return 1
	}
}

// matchEntry reads r line-by-line and writes one output row per matching line
// in the format `<path>:<line>:<text>` (or as JSON when asJSON is set).
// Returns the count of matches found in this entry.
func matchEntry(name string, r io.Reader, re *regexp.Regexp, asJSON bool, w io.Writer) int {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<16), 1<<20)
	var hits int
	enc := json.NewEncoder(w)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		if !re.MatchString(text) {
			continue
		}
		hits++
		if asJSON {
			_ = enc.Encode(map[string]any{
				"path": name,
				"line": line,
				"text": text,
			})
		} else {
			fmt.Fprintf(w, "%s:%d:%s\n", name, line, text)
		}
	}
	return hits
}

// reader-factory + open helpers — mirror the (unexported) ones in
// internal/tools/archive/archive.go so the CLI's grep mode doesn't reach
// into the package's internals.

type tarOpener func(io.Reader) (io.Reader, error)

func plainOpener(r io.Reader) (io.Reader, error) { return r, nil }
func gzipOpener(r io.Reader) (io.Reader, error)  { return gzip.NewReader(r) }
func bzip2Opener(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil }
func zstdOpener(r io.Reader) (io.Reader, error) {
	zr, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	return zr, nil
}

func grepZip(source string, re *regexp.Regexp, maxBytes int64, quiet, asJSON bool) int {
	zr, err := zip.OpenReader(source) //nolint:gosec // source comes from the user's CLI invocation
	if err != nil {
		fmt.Fprintf(os.Stderr, "inspect: open zip: %v\n", err)
		return 2
	}
	defer zr.Close()
	var totalHits, entriesMatched, entriesScanned int
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		//nolint:gosec // size-bounded by the maxBytes guard below
		if int64(f.UncompressedSize64) > maxBytes {
			if !quiet {
				fmt.Fprintf(os.Stderr, "inspect: skip %s (%d bytes > --max-bytes %d)\n", f.Name, f.UncompressedSize64, maxBytes)
			}
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			fmt.Fprintf(os.Stderr, "inspect: open entry %s: %v\n", f.Name, openErr)
			continue
		}
		entriesScanned++
		hits := matchEntry(f.Name, rc, re, asJSON, os.Stdout)
		rc.Close()
		if hits > 0 {
			entriesMatched++
			totalHits += hits
		}
	}
	finalSummary(totalHits, entriesMatched, entriesScanned, quiet)
	if totalHits == 0 {
		return 1
	}
	return 0
}

func grepTar(source string, open tarOpener, re *regexp.Regexp, maxBytes int64, quiet, asJSON bool) int {
	f, err := os.Open(source) //nolint:gosec // source comes from the user's CLI invocation
	if err != nil {
		fmt.Fprintf(os.Stderr, "inspect: open archive: %v\n", err)
		return 2
	}
	defer f.Close()
	r, oerr := open(bufio.NewReader(f))
	if oerr != nil {
		fmt.Fprintf(os.Stderr, "inspect: open compressed stream: %v\n", oerr)
		return 2
	}
	// Best-effort close on the wrapped reader for zstd which exposes a
	// finalizer; ignore on the others.
	if closer, ok := r.(io.Closer); ok {
		defer closer.Close()
	}
	tr := tar.NewReader(r)
	var totalHits, entriesMatched, entriesScanned int
	for {
		h, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			fmt.Fprintf(os.Stderr, "inspect: tar advance: %v\n", nextErr)
			return 2
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if h.Size > maxBytes {
			if !quiet {
				fmt.Fprintf(os.Stderr, "inspect: skip %s (%d bytes > --max-bytes %d)\n", h.Name, h.Size, maxBytes)
			}
			continue
		}
		entriesScanned++
		hits := matchEntry(h.Name, io.LimitReader(tr, maxBytes), re, asJSON, os.Stdout)
		if hits > 0 {
			entriesMatched++
			totalHits += hits
		}
	}
	finalSummary(totalHits, entriesMatched, entriesScanned, quiet)
	if totalHits == 0 {
		return 1
	}
	return 0
}

// finalSummary prints a final stderr line so the user sees what was scanned
// (helpful when grep produced no output). Suppressed under --quiet.
func finalSummary(totalHits, entriesMatched, entriesScanned int, quiet bool) {
	if quiet {
		return
	}
	fmt.Fprintf(os.Stderr, "inspect: %d match%s across %d entr%s (scanned %d)\n",
		totalHits, plural(totalHits, "es", "es"),
		entriesMatched, plural(entriesMatched, "ies", "y"),
		entriesScanned,
	)
}

func plural(n int, plur, sing string) string {
	if n == 1 {
		return sing
	}
	return plur
}
