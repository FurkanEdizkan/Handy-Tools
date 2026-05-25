package main

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
)

type archiveInspectInput struct {
	Source string `json:"source" jsonschema:"absolute path of the archive to inspect"`
}

type archiveExtractInput struct {
	Source        string   `json:"source" jsonschema:"absolute path of the archive to extract"`
	Destination   string   `json:"destination" jsonschema:"absolute directory to extract into"`
	Parts         []string `json:"parts,omitempty" jsonschema:"optional list of multi-part volume paths (use archive_inspect first to enumerate)"`
	Password      string   `json:"password,omitempty" jsonschema:"password for encrypted archives"`
	Overwrite     bool     `json:"overwrite,omitempty" jsonschema:"overwrite existing files in destination"`
	AutoMultiPart bool     `json:"auto_multi_part,omitempty" jsonschema:"proceed with multi-part archives without confirmation"`
}

type archiveCompressInput struct {
	Sources          []string `json:"sources" jsonschema:"absolute paths to pack (files or directories; directories are added recursively)"`
	Output           string   `json:"output" jsonschema:"absolute path of the archive to write"`
	Format           string   `json:"format,omitempty" jsonschema:"one of: zip, tar, tar.gz, tar.bz2, tar.zst, 7z; empty = infer from output extension"`
	Password         string   `json:"password,omitempty" jsonschema:"only honored by 7z"`
	CompressionLevel int      `json:"compression_level,omitempty" jsonschema:"0-9 where supported by the format"`
}

func parseArchiveFormat(s string) archive.Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return archive.FormatUnknown
	case "zip":
		return archive.FormatZip
	case "tar":
		return archive.FormatTar
	case "tar.gz", "tgz", "gz":
		return archive.FormatTarGz
	case "tar.bz2", "tbz", "tbz2", "bz2":
		return archive.FormatTarBz2
	case "tar.zst", "tzst", "zst":
		return archive.FormatTarZst
	case "7z":
		return archive.FormatSevenZ
	}
	return archive.FormatUnknown
}

func registerArchiveTools(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "archive_inspect",
		Description: "Inspect an archive: report format, multi-part status, entry count, and whether a required binary (unrar / 7z) is on PATH.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in archiveInspectInput) (*mcp.CallToolResult, any, error) {
		ins, err := h.Archive.Inspect(ctx, server.InspectParams{Source: in.Source})
		if err != nil {
			return errorResult("archive", "inspect", err), nil, nil
		}
		header := "archive.inspect: ok\n"
		return jsonResult(header, ins), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "archive_extract",
		Description: "Extract an archive into destination. zip/tar are pure-Go; RAR and 7z need the unrar / 7z binaries.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in archiveExtractInput) (*mcp.CallToolResult, any, error) {
		res := drainProgress(ctx, "archive", "extract", func(emit func(tools.Progress) error) error {
			return h.Archive.Extract(ctx, server.ExtractParams{
				Source:        in.Source,
				Parts:         in.Parts,
				Destination:   in.Destination,
				Password:      in.Password,
				Overwrite:     in.Overwrite,
				AutoMultiPart: in.AutoMultiPart,
			}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "archive_compress",
		Description: "Pack sources into a single archive. zip/tar variants are pure-Go; 7z needs the 7z binary; RAR creation is not supported.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in archiveCompressInput) (*mcp.CallToolResult, any, error) {
		res := drainProgress(ctx, "archive", "compress", func(emit func(tools.Progress) error) error {
			return h.Archive.Compress(ctx, server.CompressParams{
				Sources:          in.Sources,
				Format:           parseArchiveFormat(in.Format),
				Output:           in.Output,
				Password:         in.Password,
				CompressionLevel: in.CompressionLevel,
			}, emit)
		})
		return res, nil, nil
	})
}
