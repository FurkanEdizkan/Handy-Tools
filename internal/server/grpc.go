// File grpc.go contains the bindings between the wire-shape-agnostic
// handlers (image.go, archive.go, pdf.go) and the generated proto types
// in gen/handytools/v1.
//
// The proto types are produced by `buf generate` (Make target: `make proto`).
// CI runs that step before building, so this file always sees a populated
// gen/handytools/v1 package. To work locally, run:
//
//	make proto
//
// once after cloning, then `go build ./...`.

package server

import (
	"bytes"
	"context"
	"image/png"
	"math"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	handytoolsv1 "github.com/furkandedizkan/handy-tools/gen/handytools/v1"
	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
	"github.com/furkandedizkan/handy-tools/internal/tools/pdf"
)

// Server bundles the handlers + the gRPC server.
type Server struct {
	Opts    Options
	Image   *ImageHandler
	Archive *ArchiveHandler
	PDF     *PDFHandler

	// Queue is the shared job registry. Pass the same *queue.Queue the HTTP
	// server uses so a job started on either transport is visible to both.
	Queue *queue.Queue

	grpc *grpc.Server
}

// New constructs a server with all handlers wired in. q is the shared queue
// (see Server.Queue).
func New(opts Options, q *queue.Queue) *Server {
	return &Server{
		Opts:    opts,
		Image:   &ImageHandler{Opts: opts},
		Archive: &ArchiveHandler{Opts: opts},
		PDF:     &PDFHandler{Opts: opts},
		Queue:   q,
	}
}

// streamViaQueue enqueues a tool job on the shared queue and forwards its
// progress events onto a gRPC stream. run drives the tool; a non-nil error
// from it becomes a terminal failure Progress event, so the stream completes
// normally rather than returning a gRPC-level error. Because the queue runs
// the job under its own context, the job outlives a disconnected client.
func streamViaQueue(
	ctx context.Context,
	q *queue.Queue,
	tool, action string,
	send func(tools.Progress) error,
	run func(ctx context.Context, emit func(tools.Progress)) error,
) error {
	id := q.Enqueue(tool, action, func(jobCtx context.Context, emit func(tools.Progress)) {
		if err := run(jobCtx, emit); err != nil {
			emit(grpcFailureProgress(tool, action, err))
		}
	})
	ch, err := q.Subscribe(ctx, id)
	if err != nil {
		return err
	}
	for p := range ch {
		if err := send(p); err != nil {
			return err
		}
	}
	return nil
}

// grpcFailureProgress wraps a non-tool error into a terminal Progress event.
func grpcFailureProgress(tool, action string, err error) tools.Progress {
	te, ok := err.(*tools.Error)
	if !ok {
		te = &tools.Error{Code: tools.CodeIO, Message: err.Error()}
	}
	return tools.Progress{
		Tool:      tool,
		Action:    action,
		Level:     tools.SeverityError,
		Completed: true,
		Err:       te,
	}
}

// Serve registers the gRPC services on a new gRPC server and blocks on the
// given listener. Call Stop() from another goroutine to shut down.
func (s *Server) Serve(lis net.Listener) error {
	s.grpc = grpc.NewServer()
	handytoolsv1.RegisterImageServiceServer(s.grpc, &grpcImageServer{h: s.Image, q: s.Queue})
	handytoolsv1.RegisterArchiveServiceServer(s.grpc, &grpcArchiveServer{h: s.Archive, q: s.Queue})
	handytoolsv1.RegisterPdfServiceServer(s.grpc, &grpcPDFServer{h: s.PDF, q: s.Queue})
	reflection.Register(s.grpc)
	return s.grpc.Serve(lis)
}

// Stop gracefully halts the server.
func (s *Server) Stop() {
	if s.grpc != nil {
		s.grpc.GracefulStop()
	}
}

// ---- gRPC adapter shims -----------------------------------------------------

type grpcImageServer struct {
	handytoolsv1.UnimplementedImageServiceServer
	h *ImageHandler
	q *queue.Queue
}

func (g *grpcImageServer) Convert(req *handytoolsv1.ConvertRequest, stream handytoolsv1.ImageService_ConvertServer) error {
	out := ""
	if req.GetOutput().GetFile() != "" {
		out = req.GetOutput().GetFile()
	}
	dir := ""
	if req.GetOutput().GetDirectory() != "" {
		dir = req.GetOutput().GetDirectory()
	}
	params := ConvertParams{
		Source:       req.GetSource().GetPath(),
		TargetFormat: imageFormatFromProto(req.GetTargetFormat()),
		Quality:      int(req.GetOptions().GetQuality()),
		MaxWidth:     int(req.GetOptions().GetMaxWidth()),
		MaxHeight:    int(req.GetOptions().GetMaxHeight()),
		OutputDir:    dir,
		OutputFile:   out,
		Overwrite:    req.GetOutput().GetOverwrite(),
	}
	return streamViaQueue(stream.Context(), g.q, "image", "convert",
		func(p tools.Progress) error { return stream.Send(progressToProto(p)) },
		func(ctx context.Context, emit func(tools.Progress)) error {
			return g.h.Convert(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

// Preview renders a single downscaled thumbnail and returns the PNG bytes
// inline — no queue, no disk, no streaming.
func (g *grpcImageServer) Preview(_ context.Context, req *handytoolsv1.ImagePreviewRequest) (*handytoolsv1.ImagePreviewResponse, error) {
	src, err := g.h.Opts.CheckPath(req.GetSource().GetPath())
	if err != nil {
		return nil, err
	}
	res, err := image.Preview(image.PreviewRequest{
		Source:    src,
		MaxWidth:  int(req.GetMaxWidth()),
		MaxHeight: int(req.GetMaxHeight()),
	})
	if err != nil {
		return nil, err
	}
	return &handytoolsv1.ImagePreviewResponse{
		Image:  res.PNG,
		Width:  clampInt32(res.Width),
		Height: clampInt32(res.Height),
		Format: handytoolsv1.ImageFormat_IMAGE_FORMAT_PNG,
	}, nil
}

type grpcArchiveServer struct {
	handytoolsv1.UnimplementedArchiveServiceServer
	h *ArchiveHandler
	q *queue.Queue
}

func (g *grpcArchiveServer) Inspect(ctx context.Context, req *handytoolsv1.InspectRequest) (*handytoolsv1.InspectResponse, error) {
	ins, err := g.h.Inspect(ctx, InspectParams{Source: req.GetSource().GetPath()})
	if err != nil {
		return nil, err
	}
	parts := make([]*handytoolsv1.FileRef, 0, len(ins.DetectedParts))
	for _, p := range ins.DetectedParts {
		parts = append(parts, &handytoolsv1.FileRef{Path: p})
	}
	return &handytoolsv1.InspectResponse{
		Format:                archiveFormatToProto(ins.Format),
		MultiPart:             ins.MultiPart,
		DetectedParts:         parts,
		MissingParts:          ins.MissingParts,
		UncompressedSizeBytes: ins.UncompressedSz,
		EntryCount:            int32(ins.EntryCount), //nolint:gosec // G115: archive entry count is bounded by real archive contents, far below int32 max
		RequiresPassword:      ins.RequiresPwd,
		RequiresBinary:        ins.RequiresBinary,
		BinaryAvailable:       ins.BinaryAvailable,
	}, nil
}

func (g *grpcArchiveServer) Extract(req *handytoolsv1.ExtractRequest, stream handytoolsv1.ArchiveService_ExtractServer) error {
	parts := make([]string, 0, len(req.GetParts()))
	for _, p := range req.GetParts() {
		parts = append(parts, p.GetPath())
	}
	params := ExtractParams{
		Source:        req.GetSource().GetPath(),
		Parts:         parts,
		Destination:   req.GetDestinationDir(),
		Password:      req.GetPassword(),
		Overwrite:     req.GetOverwrite(),
		AutoMultiPart: req.GetAutoAcceptMultiPart(),
	}
	return streamViaQueue(stream.Context(), g.q, "archive", "extract",
		func(p tools.Progress) error { return stream.Send(progressToProto(p)) },
		func(ctx context.Context, emit func(tools.Progress)) error {
			return g.h.Extract(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

type grpcPDFServer struct {
	handytoolsv1.UnimplementedPdfServiceServer
	h *PDFHandler
	q *queue.Queue
}

func (g *grpcPDFServer) ToImage(req *handytoolsv1.PdfToImageRequest, stream handytoolsv1.PdfService_ToImageServer) error {
	params := PDFToImageParams{
		Source:    req.GetSource().GetPath(),
		From:      int(req.GetPages().GetFrom()),
		To:        int(req.GetPages().GetTo()),
		DPI:       int(req.GetDpi()),
		OutputDir: req.GetOutput().GetDirectory(),
		JPEG:      req.GetTargetFormat() == handytoolsv1.ImageFormat_IMAGE_FORMAT_JPEG,
	}
	return streamViaQueue(stream.Context(), g.q, "pdf", "to-image",
		func(p tools.Progress) error { return stream.Send(progressToProto(p)) },
		func(ctx context.Context, emit func(tools.Progress)) error {
			return g.h.ToImage(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

func (g *grpcPDFServer) ToText(req *handytoolsv1.PdfToTextRequest, stream handytoolsv1.PdfService_ToTextServer) error {
	params := PDFToTextParams{
		Source:     req.GetSource().GetPath(),
		From:       int(req.GetPages().GetFrom()),
		To:         int(req.GetPages().GetTo()),
		Layout:     req.GetLayout(),
		OutputFile: req.GetOutput().GetFile(),
	}
	return streamViaQueue(stream.Context(), g.q, "pdf", "to-text",
		func(p tools.Progress) error { return stream.Send(progressToProto(p)) },
		func(ctx context.Context, emit func(tools.Progress)) error {
			return g.h.ToText(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

func (g *grpcPDFServer) Merge(req *handytoolsv1.PdfMergeRequest, stream handytoolsv1.PdfService_MergeServer) error {
	srcs := make([]string, 0, len(req.GetSources()))
	for _, s := range req.GetSources() {
		srcs = append(srcs, s.GetPath())
	}
	params := PDFMergeParams{Sources: srcs, OutputFile: req.GetOutput().GetFile()}
	return streamViaQueue(stream.Context(), g.q, "pdf", "merge",
		func(p tools.Progress) error { return stream.Send(progressToProto(p)) },
		func(ctx context.Context, emit func(tools.Progress)) error {
			return g.h.Merge(ctx, params, func(p tools.Progress) error {
				emit(p)
				return nil
			})
		})
}

// Preview streams page thumbnails for the web gallery — one
// PdfPreviewResponse per page across the requested range (an empty range
// renders every page).
func (g *grpcPDFServer) Preview(req *handytoolsv1.PdfPreviewRequest, stream grpc.ServerStreamingServer[handytoolsv1.PdfPreviewResponse]) error {
	src, err := g.h.Opts.CheckPath(req.GetSource().GetPath())
	if err != nil {
		return err
	}
	res, err := pdf.Preview(stream.Context(), src, pdf.Range{
		From: int(req.GetPages().GetFrom()),
		To:   int(req.GetPages().GetTo()),
	})
	if err != nil {
		return err
	}
	for _, page := range res.Pages {
		w, h := pngBounds(page.PNG)
		if err := stream.Send(&handytoolsv1.PdfPreviewResponse{
			Page:   clampInt32(page.Page),
			Image:  page.PNG,
			Width:  clampInt32(w),
			Height: clampInt32(h),
		}); err != nil {
			return err
		}
	}
	return nil
}

// pngBounds reads a PNG's dimensions from its header without decoding pixels.
func pngBounds(b []byte) (width, height int) {
	cfg, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// clampInt32 narrows a (small, non-negative) pixel dimension into int32,
// guarding the overflow case so the conversion is provably safe.
func clampInt32(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}

// ---- proto <-> domain ------------------------------------------------------

func progressToProto(p tools.Progress) *handytoolsv1.Progress {
	startedMs := p.StartedAt.UnixMilli()
	if p.StartedAt.IsZero() {
		startedMs = time.Now().UnixMilli()
	}
	out := &handytoolsv1.Progress{
		Job: &handytoolsv1.Job{
			Id:            p.JobID,
			Tool:          p.Tool,
			Action:        p.Action,
			StartedUnixMs: startedMs,
		},
		CurrentItem: p.CurrentItem,
		BytesDone:   p.BytesDone,
		BytesTotal:  p.BytesTotal,
		Fraction:    p.Fraction,
		Estimated:   p.Estimated,
		Level:       severityToProto(p.Level),
		Message:     p.Message,
		Completed:   p.Completed,
	}
	if p.Err != nil {
		out.Error = &handytoolsv1.Error{
			Code:    p.Err.Code,
			Message: p.Err.Message,
			Detail:  p.Err.Detail,
		}
	}
	if len(p.Failures) > 0 {
		out.Failures = make([]*handytoolsv1.Failure, len(p.Failures))
		for i, f := range p.Failures {
			out.Failures[i] = &handytoolsv1.Failure{
				Path:    f.Path,
				Code:    f.Code,
				Message: f.Message,
			}
		}
	}
	return out
}

func severityToProto(s tools.Severity) handytoolsv1.Severity {
	switch s {
	case tools.SeverityInfo:
		return handytoolsv1.Severity_SEVERITY_INFO
	case tools.SeverityWarning:
		return handytoolsv1.Severity_SEVERITY_WARNING
	case tools.SeverityError:
		return handytoolsv1.Severity_SEVERITY_ERROR
	}
	return handytoolsv1.Severity_SEVERITY_UNSPECIFIED
}

func imageFormatFromProto(f handytoolsv1.ImageFormat) image.Format {
	switch f {
	case handytoolsv1.ImageFormat_IMAGE_FORMAT_PNG:
		return image.FormatPNG
	case handytoolsv1.ImageFormat_IMAGE_FORMAT_JPEG:
		return image.FormatJPEG
	case handytoolsv1.ImageFormat_IMAGE_FORMAT_GIF:
		return image.FormatGIF
	case handytoolsv1.ImageFormat_IMAGE_FORMAT_BMP:
		return image.FormatBMP
	case handytoolsv1.ImageFormat_IMAGE_FORMAT_TIFF:
		return image.FormatTIFF
	case handytoolsv1.ImageFormat_IMAGE_FORMAT_WEBP:
		return image.FormatWebP
	case handytoolsv1.ImageFormat_IMAGE_FORMAT_HEIC:
		return image.FormatHEIC
	}
	return image.FormatUnspecified
}

func archiveFormatToProto(f archive.Format) handytoolsv1.ArchiveFormat {
	switch f {
	case archive.FormatZip:
		return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_ZIP
	case archive.FormatTar:
		return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_TAR
	case archive.FormatTarGz:
		return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_GZ
	case archive.FormatTarBz2:
		return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_BZ2
	case archive.FormatTarZst:
		return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_ZST
	case archive.FormatRar:
		return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_RAR
	case archive.FormatSevenZ:
		return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_SEVENZ
	case archive.FormatTarXz:
		return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_XZ
	}
	return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_UNSPECIFIED
}
