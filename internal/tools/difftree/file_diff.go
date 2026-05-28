package difftree

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// FileDiffLimit caps the per-file read length used by FileDiff. Files larger
// than this are read up to the cap, then the diff is computed over the
// truncated content. The cap keeps the GUI responsive when the user clicks a
// row backed by a multi-megabyte log file. Exposed as a var so tests can
// shrink it without having to write 1 MiB of fixture data.
var FileDiffLimit int64 = 1 << 20 // 1 MiB

// binaryProbeBytes is how many bytes of each side are scanned for a NUL byte
// to classify a file as binary. 8 KiB matches the heuristic git uses for
// `git diff` binary detection.
const binaryProbeBytes = 8 << 10

// FileDiffLineKind tags a single diff line for the renderer.
type FileDiffLineKind string

const (
	FileDiffContext FileDiffLineKind = "context" // a context line present on both sides
	FileDiffAdd     FileDiffLineKind = "add"     // only on B
	FileDiffRemove  FileDiffLineKind = "remove"  // only on A
	FileDiffHunk    FileDiffLineKind = "hunk"    // the `@@` separator pandas
)

// FileDiffLine is one row of a unified diff, ready for UI rendering. AOld and
// BNew are 1-based line numbers on each side; 0 means "not applicable" (e.g.
// AOld on an add line, BNew on a remove line, both on a hunk marker).
type FileDiffLine struct {
	Kind FileDiffLineKind `json:"kind"`
	Text string           `json:"text"`
	AOld int              `json:"aOld,omitempty"`
	BNew int              `json:"bNew,omitempty"`
}

// FileDiffResult is the structured diff between two files.
type FileDiffResult struct {
	// Binary is true when either side contains a NUL byte in its first 8 KiB.
	// When true, Lines is nil — the renderer shows a "binary file" placeholder.
	Binary bool `json:"binary"`
	// Truncated is true when either file exceeded FileDiffLimit and the diff
	// was computed against the truncated content.
	Truncated bool `json:"truncated"`
	// Identical is true when the two files compared equal byte-for-byte
	// (within the read cap). Lines is nil in this case.
	Identical bool `json:"identical"`
	// Lines is the rendered unified diff, hunk by hunk. Empty when Identical
	// or Binary is true.
	Lines []FileDiffLine `json:"lines"`
}

// FileDiff produces a unified diff between two filesystem paths. Errors are
// returned as *tools.Error so callers can map them to a transport status
// (HTTP / MCP) consistently.
func FileDiff(a, b string) (FileDiffResult, *tools.Error) {
	aBytes, aTruncated, aerr := readCapped(a)
	if aerr != nil {
		return FileDiffResult{}, &tools.Error{Code: tools.ClassifyFSError(aerr), Message: "read A", Detail: aerr.Error()}
	}
	bBytes, bTruncated, berr := readCapped(b)
	if berr != nil {
		return FileDiffResult{}, &tools.Error{Code: tools.ClassifyFSError(berr), Message: "read B", Detail: berr.Error()}
	}
	if looksBinary(aBytes) || looksBinary(bBytes) {
		return FileDiffResult{Binary: true, Truncated: aTruncated || bTruncated}, nil
	}
	if bytes.Equal(aBytes, bBytes) {
		return FileDiffResult{Identical: true, Truncated: aTruncated || bTruncated}, nil
	}

	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(aBytes)),
		B:        difflib.SplitLines(string(bBytes)),
		FromFile: a,
		ToFile:   b,
		Context:  3,
	}
	raw, err := difflib.GetUnifiedDiffString(ud)
	if err != nil {
		return FileDiffResult{}, &tools.Error{Code: tools.CodeIO, Message: "compute diff", Detail: err.Error()}
	}
	return FileDiffResult{
		Truncated: aTruncated || bTruncated,
		Lines:     parseUnified(raw),
	}, nil
}

// readCapped reads up to FileDiffLimit+1 bytes from path, returning the bytes
// (truncated to FileDiffLimit) and whether the file exceeded the cap.
func readCapped(path string) ([]byte, bool, error) {
	f, err := os.Open(path) //nolint:gosec // caller path-checks via Opts.CheckPath
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	buf := make([]byte, FileDiffLimit+1)
	n, err := io.ReadFull(f, buf)
	switch err { //nolint:errorlint // io.ReadFull contract returns sentinel io.EOF / io.ErrUnexpectedEOF directly
	case nil:
		// Got at least limit+1 bytes — file is bigger than the cap.
		return buf[:FileDiffLimit], true, nil
	case io.EOF, io.ErrUnexpectedEOF:
		return buf[:n], false, nil
	default:
		return nil, false, err
	}
}

// looksBinary returns true if the first binaryProbeBytes bytes of b contain a
// NUL. Mirrors `git diff`'s heuristic — cheap and good enough for source-tree
// comparisons where a true text/binary boundary is ambiguous anyway.
func looksBinary(b []byte) bool {
	end := binaryProbeBytes
	if len(b) < end {
		end = len(b)
	}
	return bytes.IndexByte(b[:end], 0) >= 0
}

// parseUnified converts difflib's raw unified-diff string into structured
// FileDiffLines so the UI can render coloured rows without re-parsing
// `--- /+++` text on the client. The format is well-defined:
//
//	--- A
//	+++ B
//	@@ -L,N +L,N @@
//	 context
//	-removed
//	+added
//
// Header lines (---, +++) are dropped; the `@@` hunk header is preserved as
// kind=hunk so the renderer can show it as a separator.
func parseUnified(raw string) []FileDiffLine {
	out := make([]FileDiffLine, 0, 64)
	aLine, bLine := 0, 0
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "--- "), strings.HasPrefix(line, "+++ "):
			// File headers; we don't need them — the row already names the file.
			continue
		case strings.HasPrefix(line, "@@"):
			aLine, bLine = parseHunkHeader(line)
			out = append(out, FileDiffLine{Kind: FileDiffHunk, Text: line})
		case strings.HasPrefix(line, "+"):
			out = append(out, FileDiffLine{Kind: FileDiffAdd, Text: line[1:], BNew: bLine})
			bLine++
		case strings.HasPrefix(line, "-"):
			out = append(out, FileDiffLine{Kind: FileDiffRemove, Text: line[1:], AOld: aLine})
			aLine++
		case strings.HasPrefix(line, " "):
			out = append(out, FileDiffLine{Kind: FileDiffContext, Text: line[1:], AOld: aLine, BNew: bLine})
			aLine++
			bLine++
		}
	}
	return out
}

// parseHunkHeader pulls the starting line numbers out of "@@ -L,N +L,N @@".
// On any parse hiccup it returns (1, 1) — the renderer just labels the rows
// with whatever it gets.
func parseHunkHeader(line string) (int, int) {
	// Expect "@@ -A,a +B,b @@ optional context"
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 1, 1
	}
	a, _ := strconv.Atoi(strings.TrimPrefix(strings.SplitN(fields[1], ",", 2)[0], "-"))
	b, _ := strconv.Atoi(strings.TrimPrefix(strings.SplitN(fields[2], ",", 2)[0], "+"))
	if a <= 0 {
		a = 1
	}
	if b <= 0 {
		b = 1
	}
	return a, b
}
