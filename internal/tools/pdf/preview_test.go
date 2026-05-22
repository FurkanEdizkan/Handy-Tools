package pdf

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// twoPagePath returns the absolute path to testdata/two-page.pdf, a minimal
// hand-built two-page PDF used by the multi-page preview test.
func twoPagePath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", "two-page.pdf"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
	return p
}

// TestPreviewReportsMissingBinary confirms a stripped PATH surfaces a
// MISSING_BINARY error rather than panicking.
func TestPreviewReportsMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	sysdep.Reset()

	_, err := Preview(context.Background(), "/nonexistent.pdf", Range{})
	te, ok := err.(*tools.Error)
	if !ok || te.Code != tools.CodeMissingBinary {
		t.Fatalf("expected MISSING_BINARY, got %v", err)
	}
}

// TestPreviewSinglePage renders just page 1 of the two-page fixture and
// expects exactly one PNG page back. Skipped when pdftoppm is missing
// locally; CI installs poppler-utils so it runs there.
func TestPreviewSinglePage(t *testing.T) {
	if !sysdep.Lookup("pdftoppm").Found {
		t.Skip("pdftoppm not on PATH; skipping PDF preview test")
	}
	res, err := Preview(context.Background(), twoPagePath(t), Range{From: 1, To: 1})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(res.Pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(res.Pages))
	}
	if res.Pages[0].Page != 1 {
		t.Fatalf("page number = %d, want 1", res.Pages[0].Page)
	}
	if len(res.Pages[0].PNG) == 0 {
		t.Fatal("page 1 PNG is empty")
	}
}

// TestPreviewAllPages renders the whole two-page fixture (empty range) and
// expects both pages back, numbered 1 and 2 in order.
func TestPreviewAllPages(t *testing.T) {
	if !sysdep.Lookup("pdftoppm").Found {
		t.Skip("pdftoppm not on PATH; skipping PDF preview test")
	}
	res, err := Preview(context.Background(), twoPagePath(t), Range{})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(res.Pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(res.Pages))
	}
	for i, p := range res.Pages {
		if p.Page != i+1 {
			t.Fatalf("page[%d].Page = %d, want %d", i, p.Page, i+1)
		}
		if len(p.PNG) == 0 {
			t.Fatalf("page %d PNG is empty", p.Page)
		}
	}
}
