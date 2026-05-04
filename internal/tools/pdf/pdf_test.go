package pdf

import (
	"context"
	"os"
	"testing"

	"github.com/furkandedizkan/handy/internal/tools"
	"github.com/furkandedizkan/handy/internal/tools/sysdep"
)

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
