package main

import (
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
)

// TestParseArchiveFormat pins the MCP format vocabulary: every name the HTTP
// transport accepts (case-insensitive, dotted or underscored) resolves, and
// an unrecognized non-empty name is reported instead of silently falling
// back to extension inference.
func TestParseArchiveFormat(t *testing.T) {
	cases := map[string]archive.Format{
		"":        archive.FormatUnknown,
		"auto":    archive.FormatUnknown,
		"zip":     archive.FormatZip,
		"tar":     archive.FormatTar,
		"tar.gz":  archive.FormatTarGz,
		"TAR_GZ":  archive.FormatTarGz,
		"tgz":     archive.FormatTarGz,
		"tar.bz2": archive.FormatTarBz2,
		"tbz2":    archive.FormatTarBz2,
		"tar.zst": archive.FormatTarZst,
		"tzst":    archive.FormatTarZst,
		"tar.xz":  archive.FormatTarXz,
		"TAR_XZ":  archive.FormatTarXz,
		"txz":     archive.FormatTarXz,
		"xz":      archive.FormatTarXz,
		"7z":      archive.FormatSevenZ,
		"sevenz":  archive.FormatSevenZ,
		"rar":     archive.FormatRar,
	}
	for in, want := range cases {
		got, ok := parseArchiveFormat(in)
		if !ok || got != want {
			t.Errorf("parseArchiveFormat(%q) = %v, %v; want %v, true", in, got, ok, want)
		}
	}
	for _, bad := range []string{"tar.gzip", "zipx", "lzma"} {
		if _, ok := parseArchiveFormat(bad); ok {
			t.Errorf("parseArchiveFormat(%q) accepted an unknown format", bad)
		}
	}
}
