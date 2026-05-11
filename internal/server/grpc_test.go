package server

import (
	"testing"
	"time"

	handytoolsv1 "github.com/furkandedizkan/handy-tools/gen/v1"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
)

func TestImageFormatRoundTrip(t *testing.T) {
	cases := map[handytoolsv1.ImageFormat]image.Format{
		handytoolsv1.ImageFormat_IMAGE_FORMAT_PNG:  image.FormatPNG,
		handytoolsv1.ImageFormat_IMAGE_FORMAT_JPEG: image.FormatJPEG,
		handytoolsv1.ImageFormat_IMAGE_FORMAT_GIF:  image.FormatGIF,
		handytoolsv1.ImageFormat_IMAGE_FORMAT_BMP:  image.FormatBMP,
		handytoolsv1.ImageFormat_IMAGE_FORMAT_TIFF: image.FormatTIFF,
		handytoolsv1.ImageFormat_IMAGE_FORMAT_WEBP: image.FormatWebP,
		handytoolsv1.ImageFormat_IMAGE_FORMAT_HEIC: image.FormatHEIC,
	}
	for proto, want := range cases {
		if got := imageFormatFromProto(proto); got != want {
			t.Errorf("imageFormatFromProto(%v) = %v want %v", proto, got, want)
		}
	}
	if got := imageFormatFromProto(handytoolsv1.ImageFormat_IMAGE_FORMAT_UNSPECIFIED); got != image.FormatUnspecified {
		t.Errorf("unspecified: got %v want FormatUnspecified", got)
	}
}

func TestArchiveFormatToProto(t *testing.T) {
	cases := map[archive.Format]handytoolsv1.ArchiveFormat{
		archive.FormatZip:    handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_ZIP,
		archive.FormatTar:    handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_TAR,
		archive.FormatTarGz:  handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_GZ,
		archive.FormatTarBz2: handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_BZ2,
		archive.FormatTarZst: handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_TAR_ZST,
		archive.FormatRar:    handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_RAR,
		archive.FormatSevenZ: handytoolsv1.ArchiveFormat_ARCHIVE_FORMAT_SEVENZ,
	}
	for in, want := range cases {
		if got := archiveFormatToProto(in); got != want {
			t.Errorf("archiveFormatToProto(%v) = %v want %v", in, got, want)
		}
	}
}

func TestSeverityToProto(t *testing.T) {
	cases := map[tools.Severity]handytoolsv1.Severity{
		tools.SeverityInfo:    handytoolsv1.Severity_SEVERITY_INFO,
		tools.SeverityWarning: handytoolsv1.Severity_SEVERITY_WARNING,
		tools.SeverityError:   handytoolsv1.Severity_SEVERITY_ERROR,
	}
	for in, want := range cases {
		if got := severityToProto(in); got != want {
			t.Errorf("severityToProto(%v) = %v want %v", in, got, want)
		}
	}
}

func TestProgressToProtoFillsStartedAt(t *testing.T) {
	// Pass a zero StartedAt — converter must fill it from time.Now to keep
	// downstream consumers from reading 1970-01-01 timestamps.
	p := tools.Progress{Tool: "test", Action: "noop"}
	out := progressToProto(p)
	if out.Job == nil {
		t.Fatal("Job is nil")
	}
	if out.Job.StartedUnixMs == 0 {
		t.Fatal("StartedUnixMs not populated for zero StartedAt input")
	}
	if got := time.UnixMilli(out.Job.StartedUnixMs); time.Since(got) > time.Minute {
		t.Errorf("StartedUnixMs %v is implausible", got)
	}
}

func TestProgressToProtoCarriesError(t *testing.T) {
	p := tools.Progress{Err: &tools.Error{Code: "X", Message: "boom", Detail: "details"}}
	out := progressToProto(p)
	if out.Error == nil {
		t.Fatal("Error field not set")
	}
	if out.Error.Code != "X" || out.Error.Message != "boom" || out.Error.Detail != "details" {
		t.Errorf("error fields: %+v", out.Error)
	}
}
