package main

import (
	"fmt"
	"io"
)

const usage = `htools — Handy Tools CLI

Usage:
  htools <command> [flags] [args...]

Commands:
  convert <src>...   Convert images. Flags:
                       --format jpeg|png|webp|gif|bmp|tiff|heic   (required)
                       --quality N            JPEG quality 1..100  (default 90)
                       --max-width N          downscale, 0 = keep
                       --max-height N         downscale, 0 = keep
                       --strip-metadata
                       --out PATH             file (single src) or dir
                       --overwrite            replace if output exists
                       --quiet --json

  pack <src>...      Create one archive from one or more files/folders.
                       --format zip|tar|tar.gz|tar.bz2|tar.zst|7z (required)
                       --output FILE          (required)
                       --compression-level N  1 fastest .. 9 smallest
                       --password STR         (7z only; pure-Go formats unsupported)
                       --quiet --json

  extract <archive>  Extract a single archive into a directory.
                       --out DIR              (default: cwd)
                       --password STR
                       --auto-multi-part      proceed without confirmation on .partN
                       --overwrite
                       --quiet --json

  inspect <archive>  Show archive metadata (format, entry count, size). With
                     --grep <regex>, stream <path>:<line>:<text> matches by
                     reading entries without extracting them. Pure-Go formats
                     only for --grep (zip / tar / tar.gz / tar.bz2 / tar.zst).
                       --grep REGEX           search inside text entries
                       --max-bytes N          skip entries larger than N (default 1 MiB)
                       --quiet --json

  pdf merge <files>... --out FILE
  pdf split <src>      --pages FROM-TO --out DIR       (or --every N)
  pdf render <src>     --pages FROM-TO --dpi N [--jpeg] --out DIR
  pdf text <src>       --pages FROM-TO [--layout] --out FILE

  rename <dir>       Batch-rename files whose basename matches a regex.
                       --pattern REGEX        matched against each basename (required)
                       --replace TEMPLATE     Go regexp template, $1..$9 (required)
                       --recurse              descend into subdirectories
                       --on-collision MODE    error|skip|suffix (default error)
                       --dry-run              print the plan, do not move
                       --quiet --json

  hash <files>...    Compute file digests (sha256sum-format output).
                       --algo md5|sha256|blake3  default sha256
                       --check MANIFEST       verify a manifest instead of hashing
                       --quiet --json

  diff-tree <a> <b>  Compare two directory trees and report added / removed /
                     changed files. Symlinks are not followed.
                       --by mtime|hash        compare strategy (default mtime)
                       --quiet --json
                     Exit codes: 0 trees match, 1 trees differ, 2 I/O error.

  strip-meta <files>... Re-encode images, dropping EXIF / IPTC / XMP.
                       --in-place             overwrite source (default: write
                                              <name>-stripped.<ext> sibling)
                       --quality N            JPEG re-encode quality 1..100
                       --quiet --json

  doctor             List optional system binaries Handy Tools can use and
                     whether each one is on PATH.

  version            Print the build version.
  help               Print this help.

Common flags:
  --quiet            Suppress per-event progress lines (terminal summary stays).
  --json             Emit one JSON object per progress event on stdout.

Exit codes:
  0   success
  1   user/usage error or known tool error (bad request, missing binary)
  2   I/O error or aborted operation
`

func printUsage(w io.Writer) {
	fmt.Fprint(w, usage)
}
