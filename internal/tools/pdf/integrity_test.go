package pdf

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func terminalOf(t *testing.T, ch <-chan tools.Progress) tools.Progress {
	t.Helper()
	events := collect(ch)
	last := events[len(events)-1]
	if !last.Completed {
		t.Fatalf("stream ended without a Completed event: %+v", last)
	}
	return last
}

// copyFixture copies testdata/two-page.pdf to dir/name.
func copyFixture(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "two-page.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func noTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".handy-tmp-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// TestMergeRejectsOutputThatIsASource: `merge --out a.pdf a.pdf b.pdf` used
// to truncate a.pdf before reading it, then delete it when the read failed.
func TestMergeRejectsOutputThatIsASource(t *testing.T) {
	dir := t.TempDir()
	a := copyFixture(t, dir, "a.pdf")
	b := copyFixture(t, dir, "b.pdf")
	original, _ := os.ReadFile(a)
	link := filepath.Join(dir, "link.pdf")
	if err := os.Symlink(a, link); err != nil {
		t.Fatal(err)
	}

	for _, out := range []string{a, filepath.Join(dir, ".", "a.pdf"), link} {
		last := terminalOf(t, Merge(context.Background(), MergeRequest{Sources: []string{a, b}, OutputFile: out}))
		if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
			t.Fatalf("out=%s: want BAD_REQUEST, got %+v", out, last.Err)
		}
		if got, _ := os.ReadFile(a); !bytes.Equal(got, original) {
			t.Fatalf("out=%s: source a.pdf was modified or deleted", out)
		}
	}
}

// TestMergeFailureKeepsExistingOutput: a failed merge must leave a
// pre-existing output file alone and report the classified source error.
func TestMergeFailureKeepsExistingOutput(t *testing.T) {
	dir := t.TempDir()
	a := copyFixture(t, dir, "a.pdf")
	out := filepath.Join(dir, "merged.pdf")
	if err := os.WriteFile(out, []byte("previous merge"), 0o644); err != nil {
		t.Fatal(err)
	}

	last := terminalOf(t, Merge(context.Background(), MergeRequest{
		Sources: []string{a, filepath.Join(dir, "missing.pdf")}, OutputFile: out,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeNotFound {
		t.Fatalf("want NOT_FOUND for a missing source, got %+v", last.Err)
	}
	if got, _ := os.ReadFile(out); string(got) != "previous merge" {
		t.Fatalf("existing output was clobbered by a failed merge: %q", got)
	}
	noTempFiles(t, dir)
}

// TestMergeWritesAtomically: a successful merge still lands at OutputFile
// with every page, and no temp file is left next to it.
func TestMergeWritesAtomically(t *testing.T) {
	dir := t.TempDir()
	a := copyFixture(t, dir, "a.pdf")
	b := copyFixture(t, dir, "b.pdf")
	out := filepath.Join(dir, "merged.pdf")
	if err := os.WriteFile(out, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	last := terminalOf(t, Merge(context.Background(), MergeRequest{Sources: []string{a, b}, OutputFile: out}))
	if last.Err != nil {
		t.Fatalf("merge: %+v", last.Err)
	}
	n, err := api.PageCountFile(out)
	if err != nil || n != 4 {
		t.Fatalf("merged page count = %d (err %v), want 4", n, err)
	}
	noTempFiles(t, dir)
}

// TestSplitOpenStartRange: "-2" (CLI) / from=0 (MCP) means "from the first
// page"; it used to be sent to pdfcpu as page 0 and fail.
func TestSplitOpenStartRange(t *testing.T) {
	dir := t.TempDir()
	src := copyFixture(t, dir, "doc.pdf")
	out := filepath.Join(dir, "out")
	last := terminalOf(t, Split(context.Background(), SplitRequest{
		Source: src, PageRanges: []Range{{From: 0, To: 1}}, OutputDir: out,
	}))
	if last.Err != nil {
		t.Fatalf("split from=0 to=1: %+v", last.Err)
	}
	n, err := api.PageCountFile(filepath.Join(out, "doc_1.pdf"))
	if err != nil || n != 1 {
		t.Fatalf("page count = %d (err %v), want 1", n, err)
	}
}

func TestSplitRejectsReversedRange(t *testing.T) {
	dir := t.TempDir()
	src := copyFixture(t, dir, "doc.pdf")
	out := filepath.Join(dir, "out")
	last := terminalOf(t, Split(context.Background(), SplitRequest{
		Source: src, PageRanges: []Range{{From: 1, To: 2}, {From: 2, To: 1}}, OutputDir: out,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("want BAD_REQUEST, got %+v", last.Err)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Fatalf("a rejected request still wrote output: %v", entries)
	}
}

func TestSplitMissingSourceIsNotFound(t *testing.T) {
	dir := t.TempDir()
	last := terminalOf(t, Split(context.Background(), SplitRequest{
		Source: filepath.Join(dir, "missing.pdf"), PageRanges: []Range{{From: 1, To: 1}}, OutputDir: dir,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeNotFound {
		t.Fatalf("page-ranges mode: want NOT_FOUND, got %+v", last.Err)
	}
	last = terminalOf(t, Split(context.Background(), SplitRequest{
		Source: filepath.Join(dir, "missing.pdf"), EveryN: 1, OutputDir: dir,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeNotFound {
		t.Fatalf("every-n mode: want NOT_FOUND, got %+v", last.Err)
	}
}

func TestRangeSelectorOpenStart(t *testing.T) {
	cases := []struct {
		in   Range
		want string
	}{
		{Range{From: 0, To: 5}, "1-5"},
		{Range{From: 0, To: 0}, "1-"},
		{Range{From: -3, To: 2}, "1-2"},
		{Range{From: 0, To: 1}, "1"},
	}
	for _, c := range cases {
		if got := rangeSelector(c.in); got != c.want {
			t.Errorf("rangeSelector(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}
