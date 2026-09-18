package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	handytoolsv1 "github.com/furkandedizkan/handy-tools/gen/handytools/v1"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
)

// TestCheckPathRejectionIsBadRequest pins the sandbox-rejection code so every
// transport (HTTP sync + async, gRPC, MCP) reports the same thing.
func TestCheckPathRejectionIsBadRequest(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"", "/etc/passwd"} {
		_, err := Options{AllowRoots: []string{root}}.CheckPath(p)
		var te *tools.Error
		if !errors.As(err, &te) || te.Code != tools.CodeBadRequest {
			t.Errorf("CheckPath(%q) = %v, want a %s *tools.Error", p, err, tools.CodeBadRequest)
		}
	}
	// No roots at all still fails closed with the same code.
	_, err := Options{}.CheckPath("/tmp/x")
	var te *tools.Error
	if !errors.As(err, &te) || te.Code != tools.CodeBadRequest {
		t.Errorf("fail-closed CheckPath = %v, want %s", err, tools.CodeBadRequest)
	}
}

func TestToolErrorClassifiesPlainErrors(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&tools.Error{Code: tools.CodeMissingBinary, Message: "x"}, tools.CodeMissingBinary},
		{fmt.Errorf("wrapped: %w", &tools.Error{Code: tools.CodeNotFound}), tools.CodeNotFound},
		{&fs.PathError{Op: "stat", Path: "/nope", Err: fs.ErrNotExist}, tools.CodeNotFound},
		{&fs.PathError{Op: "open", Path: "/root", Err: fs.ErrPermission}, tools.CodePermissionDenied},
		{errors.New("zip: not a valid zip file"), tools.CodeBadRequest},
		// A disk error is the server's failure, not a bad request.
		{&fs.PathError{Op: "read", Path: "/data/a.png", Err: syscall.EIO}, tools.CodeIO},
		{&os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.ENOSPC}, tools.CodeIO},
	}
	for _, c := range cases {
		if got := ToolError(c.err); got == nil || got.Code != c.want {
			t.Errorf("ToolError(%v) = %+v, want code %s", c.err, got, c.want)
		}
	}
	if ToolError(nil) != nil {
		t.Error("ToolError(nil) should be nil")
	}
}

// TestForwardProgressReportsAbortWhenTerminalDropped: when a tool closes its
// channel without a Completed event because ctx was canceled (its emit
// selects on ctx.Done and can drop the terminal event), the handler must
// return ABORTED instead of nil so the job is not recorded as "done".
func TestForwardProgressReportsAbortWhenTerminalDropped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := make(chan tools.Progress, 1)
	ch <- tools.Progress{Message: "[1/9] wrote a.png"}
	close(ch)

	var got []tools.Progress
	err := forwardProgress(ctx, ch, func(p tools.Progress) error {
		got = append(got, p)
		return nil
	})
	var te *tools.Error
	if !errors.As(err, &te) || te.Code != tools.CodeAborted {
		t.Fatalf("forwardProgress = %v, want an %s *tools.Error", err, tools.CodeAborted)
	}
	if len(got) != 1 {
		t.Fatalf("forwarded %d events, want 1", len(got))
	}
}

func TestForwardProgressNilWhenTerminalSeen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled, but the tool still managed to deliver its terminal event
	ch := make(chan tools.Progress, 1)
	ch <- tools.Progress{Completed: true, Err: &tools.Error{Code: tools.CodeAborted}}
	close(ch)
	if err := forwardProgress(ctx, ch, func(tools.Progress) error { return nil }); err != nil {
		t.Fatalf("forwardProgress = %v, want nil when a terminal event was forwarded", err)
	}
	ch2 := make(chan tools.Progress)
	close(ch2)
	if err := forwardProgress(context.Background(), ch2, func(tools.Progress) error { return nil }); err != nil {
		t.Fatalf("forwardProgress on a live ctx = %v, want nil", err)
	}
}

// TestCanceledBatchNeverEndsSilently drives a real handler with a canceled
// context many times: every run must end with either a forwarded terminal
// event or an ABORTED error — never a silent nil that reads as success.
func TestCanceledBatchNeverEndsSilently(t *testing.T) {
	root := t.TempDir()
	var srcs []string
	for i := 0; i < 16; i++ {
		p := filepath.Join(root, fmt.Sprintf("in%02d.png", i))
		writeGRPCTestPNG(t, p, 4, 4)
		srcs = append(srcs, p)
	}
	out := filepath.Join(root, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	h := &ImageHandler{Opts: Options{AllowRoots: []string{root}}}
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		sawTerminal := false
		err := h.BatchConvert(ctx, BatchConvertParams{
			Sources: srcs, TargetFormat: image.FormatPNG, OutputDir: out, Overwrite: true,
		}, func(p tools.Progress) error {
			if p.Completed {
				sawTerminal = true
			}
			return nil
		})
		var te *tools.Error
		if !sawTerminal && (!errors.As(err, &te) || te.Code != tools.CodeAborted) {
			t.Fatalf("run %d: no terminal event and err=%v; a canceled job must report ABORTED", i, err)
		}
	}
}

// TestGRPCUnaryErrorsCarryStatusCodes: unary RPC failures must map the tool
// error code onto a gRPC status code instead of collapsing to Unknown.
func TestGRPCUnaryErrorsCarryStatusCodes(t *testing.T) {
	root := t.TempDir()
	opts := Options{AllowRoots: []string{root}}
	ctx := context.Background()

	ig := &grpcImageServer{h: &ImageHandler{Opts: opts}}
	_, err := ig.Preview(ctx, &handytoolsv1.ImagePreviewRequest{
		Source: &handytoolsv1.FileRef{Path: filepath.Join(root, "missing.png")},
	})
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("image preview of a missing file: code %v (%v), want NotFound", got, err)
	}
	_, err = ig.Preview(ctx, &handytoolsv1.ImagePreviewRequest{
		Source: &handytoolsv1.FileRef{Path: "/etc/hostname"},
	})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("image preview outside roots: code %v (%v), want InvalidArgument", got, err)
	}

	ag := &grpcArchiveServer{h: &ArchiveHandler{Opts: opts}}
	_, err = ag.Inspect(ctx, &handytoolsv1.InspectRequest{
		Source: &handytoolsv1.FileRef{Path: filepath.Join(root, "missing.zip")},
	})
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("archive inspect of a missing file: code %v (%v), want NotFound", got, err)
	}
}

func TestGRPCCodeMapping(t *testing.T) {
	cases := map[string]codes.Code{
		tools.CodeBadRequest:       codes.InvalidArgument,
		tools.CodeUnsupportedInput: codes.InvalidArgument,
		tools.CodeMissingBinary:    codes.FailedPrecondition,
		tools.CodePermissionDenied: codes.PermissionDenied,
		tools.CodeNotFound:         codes.NotFound,
		tools.CodeAborted:          codes.Canceled,
		tools.CodeIO:               codes.Internal,
		tools.CodeRollbackFailed:   codes.Internal,
	}
	for in, want := range cases {
		err := grpcError(&tools.Error{Code: in, Message: "m"})
		if got := status.Code(err); got != want {
			t.Errorf("grpcError(%s) code = %v, want %v", in, got, want)
		}
	}
	if grpcError(nil) != nil {
		t.Error("grpcError(nil) should be nil")
	}
}
