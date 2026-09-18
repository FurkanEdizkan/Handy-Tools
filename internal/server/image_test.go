package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
)

// TestImageConvertRejectsOutputFileOutsideRoot guards the OutputFile branch
// of ImageHandler.Convert: an explicit output file must go through the same
// allow-root sandbox as the source and OutputDir, on every transport.
func TestImageConvertRejectsOutputFileOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	src := filepath.Join(root, "in.png")
	writeGRPCTestPNG(t, src, 4, 4)
	target := filepath.Join(outside, "escaped.png")

	h := &ImageHandler{Opts: Options{AllowRoots: []string{root}}}
	err := h.Convert(context.Background(), ConvertParams{
		Source:       src,
		TargetFormat: image.FormatPNG,
		OutputFile:   target,
	}, func(tools.Progress) error { return nil })
	if err == nil {
		t.Fatal("expected a CheckPath rejection for an OutputFile outside the allow-root")
	}
	var te *tools.Error
	if !errors.As(err, &te) || te.Code != tools.CodeBadRequest {
		t.Fatalf("error = %v, want a %s *tools.Error", err, tools.CodeBadRequest)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("output written outside the allow-root (stat err=%v)", statErr)
	}
}

// TestImageConvertAllowsOutputFileInsideRoot is the positive control: an
// OutputFile under the root is accepted and written.
func TestImageConvertAllowsOutputFileInsideRoot(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in.png")
	writeGRPCTestPNG(t, src, 4, 4)
	target := filepath.Join(root, "out.png")

	h := &ImageHandler{Opts: Options{AllowRoots: []string{root}}}
	var terminal *tools.Progress
	err := h.Convert(context.Background(), ConvertParams{
		Source:       src,
		TargetFormat: image.FormatPNG,
		OutputFile:   target,
	}, func(p tools.Progress) error {
		if p.Completed {
			terminal = &p
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if terminal == nil || terminal.Err != nil {
		t.Fatalf("terminal event = %+v, want success", terminal)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("output missing: %v", statErr)
	}
}
