// File grpc.go contains the bindings between the wire-shape-agnostic
// handlers (image.go, archive.go, pdf.go) and the generated proto types
// in gen/handy/v1.
//
// The proto types are produced by `buf generate` (Make target: `make proto`).
// CI runs that step before building, so this file always sees a populated
// gen/handy/v1 package. To work locally, run:
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

	handyv1 "github.com/furkandedizkan/handy/gen/handy/v1"
	"github.com/furkandedizkan/handy/internal/tools"
	"github.com/furkandedizkan/handy/internal/tools/archive"
	"github.com/furkandedizkan/handy/internal/tools/image"
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
	handyv1.RegisterImageServiceServer(s.grpc, &grpcImageServer{h: s.Image})
	handyv1.RegisterArchiveServiceServer(s.grpc, &grpcArchiveServer{h: s.Archive})
	handyv1.RegisterPdfServiceServer(s.grpc, &grpcPDFServer{h: s.PDF})
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
	handyv1.UnimplementedImageServiceServer
	h *ImageHandler
}

func (g *grpcImageServer) Convert(req *handyv1.ConvertRequest, stream handyv1.ImageService_ConvertServer) error {
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
	handyv1.UnimplementedArchiveServiceServer
	h *ArchiveHandler
}

func (g *grpcArchiveServer) Inspect(ctx context.Context, req *handyv1.InspectRequest) (*handyv1.InspectResponse, error) {
	ins, err := g.h.Inspect(ctx, InspectParams{Source: req.GetSource().GetPath()})
	if err != nil {
		return nil, err
	}
	parts := make([]*handyv1.FileRef, 0, len(ins.DetectedParts))
	for _, p := range ins.DetectedParts {
		parts = append(parts, &handyv1.FileRef{Path: p})
	}
	return &handyv1.InspectResponse{
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

func (g *grpcArchiveServer) Extract(req *handyv1.ExtractRequest, stream handyv1.ArchiveService_ExtractServer) error {
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
	handyv1.UnimplementedPdfServiceServer
	h *PDFHandler
}

func (g *grpcPDFServer) ToImage(req *handyv1.PdfToImageRequest, stream handyv1.PdfService_ToImageServer) error {
	params := PDFToImageParams{
		Source:    req.GetSource().GetPath(),
		From:      int(req.GetPages().GetFrom()),
		To:        int(req.GetPages().GetTo()),
		DPI:       int(req.GetDpi()),
		OutputDir: req.GetOutput().GetDirectory(),
		JPEG:      req.GetTargetFormat() == handyv1.ImageFormat_IMAGE_FORMAT_JPEG,
	}
	return g.h.ToImage(stream.Context(), params, func(p tools.Progress) error {
		return stream.Send(progressToProto(p))
	})
}

func (g *grpcPDFServer) ToText(req *handyv1.PdfToTextRequest, stream handyv1.PdfService_ToTextServer) error {
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

func (g *grpcPDFServer) Merge(req *handyv1.PdfMergeRequest, stream handyv1.PdfService_MergeServer) error {
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

func progressToProto(p tools.Progress) *handyv1.Progress {
	out := &handyv1.Progress{
		Job: &handyv1.Job{
			Id:            p.JobID,
			Tool:          p.Tool,
			Action:        p.Action,
			StartedUnixMs: p.StartedAt.UnixMilli(),
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
		out.Error = &handyv1.Error{
			Code:    p.Err.Code,
			Message: p.Err.Message,
			Detail:  p.Err.Detail,
		}
	}
	if out.Job.StartedUnixMs == 0 {
		out.Job.StartedUnixMs = time.Now().UnixMilli()
	}
	return out
}

func severityToProto(s tools.Severity) handyv1.Severity {
	switch s {
	case tools.SeverityInfo:
		return handyv1.Severity_SEVERITY_INFO
	case tools.SeverityWarning:
		return handyv1.Severity_SEVERITY_WARNING
	case tools.SeverityError:
		return handyv1.Severity_SEVERITY_ERROR
	}
	return handyv1.Severity_SEVERITY_UNSPECIFIED
}

func imageFormatFromProto(f handyv1.ImageFormat) image.Format {
	switch f {
	case handyv1.ImageFormat_IMAGE_FORMAT_PNG:
		return image.FormatPNG
	case handyv1.ImageFormat_IMAGE_FORMAT_JPEG:
		return image.FormatJPEG
	case handyv1.ImageFormat_IMAGE_FORMAT_GIF:
		return image.FormatGIF
	case handyv1.ImageFormat_IMAGE_FORMAT_BMP:
		return image.FormatBMP
	case handyv1.ImageFormat_IMAGE_FORMAT_TIFF:
		return image.FormatTIFF
	case handyv1.ImageFormat_IMAGE_FORMAT_WEBP:
		return image.FormatWebP
	case handyv1.ImageFormat_IMAGE_FORMAT_HEIC:
		return image.FormatHEIC
	}
	return image.FormatUnspecified
}

func archiveFormatToProto(f archive.Format) handyv1.ArchiveFormat {
	switch f {
	case archive.FormatZip:
		return handyv1.ArchiveFormat_ARCHIVE_FORMAT_ZIP
	case archive.FormatTar:
		return handyv1.ArchiveFormat_ARCHIVE_FORMAT_TAR
	case archive.FormatTarGz:
		return handyv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_GZ
	case archive.FormatTarBz2:
		return handyv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_BZ2
	case archive.FormatTarZst:
		return handyv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_ZST
	case archive.FormatRar:
		return handyv1.ArchiveFormat_ARCHIVE_FORMAT_RAR
	case archive.FormatSevenZ:
		return handyv1.ArchiveFormat_ARCHIVE_FORMAT_SEVENZ
	}
	return handyv1.ArchiveFormat_ARCHIVE_FORMAT_UNSPECIFIED
}
