// File grpc.go contains the bindings between the wire-shape-agnostic
// handlers (image.go, archive.go, pdf.go) and the generated proto types
// in gen/v1.
//
// The proto types are produced by `buf generate` (Make target: `make proto`).
// CI runs that step before building, so this file always sees a populated
// gen/v1 package. To work locally, run:
//
//	make proto
//
// once after cloning, then `go build ./...`.

package server

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	handytoolsv1 "github.com/furkandedizkan/handy-tools/gen/v1"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
)

// Server bundles the handlers + the gRPC server.
type Server struct {
	Opts    Options
	Image   *ImageHandler
	Archive *ArchiveHandler
	PDF     *PDFHandler

	grpc *grpc.Server
}

// New constructs a server with all handlers wired in.
func New(opts Options) *Server {
	return &Server{
		Opts:    opts,
		Image:   &ImageHandler{Opts: opts},
		Archive: &ArchiveHandler{Opts: opts},
		PDF:     &PDFHandler{Opts: opts},
	}
}

// Serve registers the gRPC services on a new gRPC server and blocks on the
// given listener. Call Stop() from another goroutine to shut down.
func (s *Server) Serve(lis net.Listener) error {
	s.grpc = grpc.NewServer()
	handytoolsv1.RegisterImageServiceServer(s.grpc, &grpcImageServer{h: s.Image})
	handytoolsv1.RegisterArchiveServiceServer(s.grpc, &grpcArchiveServer{h: s.Archive})
	handytoolsv1.RegisterPdfServiceServer(s.grpc, &grpcPDFServer{h: s.PDF})
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
	return g.h.Convert(stream.Context(), params, func(p tools.Progress) error {
		return stream.Send(progressToProto(p))
	})
}

type grpcArchiveServer struct {
	handytoolsv1.UnimplementedArchiveServiceServer
	h *ArchiveHandler
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
		Format:                 archiveFormatToProto(ins.Format),
		MultiPart:              ins.MultiPart,
		DetectedParts:          parts,
		MissingParts:           ins.MissingParts,
		UncompressedSizeBytes:  ins.UncompressedSz,
		EntryCount:             int32(ins.EntryCount),
		RequiresPassword:       ins.RequiresPwd,
		RequiresBinary:         ins.RequiresBinary,
		BinaryAvailable:        ins.BinaryAvailable,
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
	return g.h.Extract(stream.Context(), params, func(p tools.Progress) error {
		return stream.Send(progressToProto(p))
	})
}

type grpcPDFServer struct {
	handytoolsv1.UnimplementedPdfServiceServer
	h *PDFHandler
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
	return g.h.ToImage(stream.Context(), params, func(p tools.Progress) error {
		return stream.Send(progressToProto(p))
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
	return g.h.ToText(stream.Context(), params, func(p tools.Progress) error {
		return stream.Send(progressToProto(p))
	})
}

func (g *grpcPDFServer) Merge(req *handytoolsv1.PdfMergeRequest, stream handytoolsv1.PdfService_MergeServer) error {
	srcs := make([]string, 0, len(req.GetSources()))
	for _, s := range req.GetSources() {
		srcs = append(srcs, s.GetPath())
	}
	params := PDFMergeParams{Sources: srcs, OutputFile: req.GetOutput().GetFile()}
	return g.h.Merge(stream.Context(), params, func(p tools.Progress) error {
		return stream.Send(progressToProto(p))
	})
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
	}
	return handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_UNSPECIFIED
}
