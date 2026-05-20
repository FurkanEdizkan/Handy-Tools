package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// fixturePath returns the absolute path to testdata/tiny.pdf, the minimal
// hand-built PDF used by the round-trip tests below.
func fixturePath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", "tiny.pdf"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
	return p
}

func collect(ch <-chan tools.Progress) []tools.Progress {
	out := []tools.Progress{}
	for p := range ch {
		out = append(out, p)
	}
	return out
}

func TestToImageReportsMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	sysdep.Reset()

	dir := t.TempDir()
	prog := collect(ToImage(context.Background(), ToImageRequest{
		Source:    "/nonexistent.pdf",
		OutputDir: dir,
	}))
	last := prog[len(prog)-1]
	if last.Err == nil || last.Err.Code != tools.CodeMissingBinary {
		t.Fatalf("expected MISSING_BINARY, got %+v", last)
	}
}

func TestMergeRejectsTooFewSources(t *testing.T) {
	prog := collect(Merge(context.Background(), MergeRequest{
		Sources:    []string{"/a.pdf"},
		OutputFile: "/out.pdf",
	}))
	last := prog[len(prog)-1]
	if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST, got %+v", last)
	}
}

func TestSplitRejectsAmbiguousMode(t *testing.T) {
	// Neither mode set.
	last := collect(Split(context.Background(), SplitRequest{
		Source: "/a.pdf", OutputDir: "/out",
	}))
	if l := last[len(last)-1]; l.Err == nil || l.Err.Code != tools.CodeBadRequest {
		t.Fatalf("no mode: expected BAD_REQUEST, got %+v", l)
	}
	// Both modes set.
	last = collect(Split(context.Background(), SplitRequest{
		Source: "/a.pdf", OutputDir: "/out",
		EveryN: 2, PageRanges: []Range{{From: 1, To: 3}},
	}))
	if l := last[len(last)-1]; l.Err == nil || l.Err.Code != tools.CodeBadRequest {
		t.Fatalf("both modes: expected BAD_REQUEST, got %+v", l)
	}
}

func TestSplitReportsMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	last := collect(Split(context.Background(), SplitRequest{
		Source: "/nonexistent.pdf", OutputDir: t.TempDir(), EveryN: 1,
	}))
	if l := last[len(last)-1]; l.Err == nil || l.Err.Code != tools.CodeMissingBinary {
		t.Fatalf("expected MISSING_BINARY, got %+v", l)
	}
}

func TestRangeSelector(t *testing.T) {
	cases := []struct {
		in   Range
		want string
	}{
		{Range{From: 2, To: 5}, "2-5"},
		{Range{From: 3, To: 0}, "3-"},
		{Range{From: 4, To: 4}, "4"},
	}
	for _, c := range cases {
		if got := rangeSelector(c.in); got != c.want {
			t.Errorf("rangeSelector(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// guards against accidental change that swallows the cleanup
func TestProgressChannelCloses(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	sysdep.Reset()

	ch := ToText(context.Background(), ToTextRequest{Source: "/nope.pdf"})
	if _, err := os.Stat("/nope.pdf"); err == nil {
		t.Skip("dev quirk: /nope.pdf actually exists")
	}
	for range ch {
	}
}

// TestToImageRendersPNG renders the checked-in fixture PDF to PNG and asserts
// at least one image file appears in the output directory. Skipped when
// pdftoppm is missing locally; CI installs poppler-utils so it runs there.
func TestToImageRendersPNG(t *testing.T) {
	sysdep.Reset()
	if !sysdep.Lookup("pdftoppm").Found {
		t.Skip("pdftoppm not on PATH; skipping PDF render test")
	}
	src := fixturePath(t)
	out := t.TempDir()

	last := collect(ToImage(context.Background(), ToImageRequest{
		Source: src, OutputDir: out, OutputBase: "page", DPI: 72,
	}))
	if l := last[len(last)-1]; l.Err != nil {
		t.Fatalf("render: %v", l.Err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("read out dir: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "page") && strings.HasSuffix(e.Name(), ".png") {
			info, _ := e.Info()
			if info != nil && info.Size() > 0 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected a non-empty page-*.png in %s, found %v", out, entries)
	}
}

// TestToTextExtractsContent extracts text from the fixture PDF and asserts the
// known string "Hi Handy" is present in the output file. Skipped when
// pdftotext is missing locally.
func TestToTextExtractsContent(t *testing.T) {
	sysdep.Reset()
	if !sysdep.Lookup("pdftotext").Found {
		t.Skip("pdftotext not on PATH; skipping PDF text extraction test")
	}
	src := fixturePath(t)
	outFile := filepath.Join(t.TempDir(), "out.txt")

	last := collect(ToText(context.Background(), ToTextRequest{
		Source: src, OutputFile: outFile,
	}))
	if l := last[len(last)-1]; l.Err != nil {
		t.Fatalf("extract: %v", l.Err)
	}
	body, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(body), "Hi Handy") {
		t.Fatalf("expected 'Hi Handy' in extracted text, got %q", string(body))
	}
}
