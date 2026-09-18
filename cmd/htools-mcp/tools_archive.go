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
	Format           string   `json:"format,omitempty" jsonschema:"one of: zip, tar, tar.gz, tar.bz2, tar.zst, tar.xz, 7z; empty = infer from output extension"`
	Password         string   `json:"password,omitempty" jsonschema:"only honored by 7z"`
	CompressionLevel int      `json:"compression_level,omitempty" jsonschema:"0-9 where supported by the format"`
	Overwrite        bool     `json:"overwrite,omitempty" jsonschema:"true: replace an existing file at output; false (default): rename to <stem>-1.<ext>"`
}

// parseArchiveFormat maps the MCP format string onto archive.Format. Empty
// or "auto" means "infer from the output extension" (FormatUnknown, true).
// Names are case-insensitive and accept both the dotted MCP spelling
// ("tar.gz") and the HTTP enum spelling ("TAR_GZ"). An unrecognized
// non-empty name returns ok=false so the caller can reject it rather than
// silently inferring a different format. RAR is accepted so archive.Compress
// can reject it with its own UNSUPPORTED_INPUT, as over HTTP.
func parseArchiveFormat(s string) (archive.Format, bool) {
	switch strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "_", ".") {
	case "", "auto":
		return archive.FormatUnknown, true
	case "zip":
		return archive.FormatZip, true
	case "tar":
		return archive.FormatTar, true
	case "tar.gz", "tgz", "gz":
		return archive.FormatTarGz, true
	case "tar.bz2", "tbz", "tbz2", "bz2":
		return archive.FormatTarBz2, true
	case "tar.zst", "tzst", "zst":
		return archive.FormatTarZst, true
	case "tar.xz", "txz", "xz":
		return archive.FormatTarXz, true
	case "7z", "sevenz":
		return archive.FormatSevenZ, true
	case "rar":
		return archive.FormatRar, true
	}
	return archive.FormatUnknown, false
}

// unknownFormatError is the BAD_REQUEST for an unrecognized format name,
// matching the HTTP transport's response.
func unknownFormatError(format string) *tools.Error {
	return &tools.Error{Code: tools.CodeBadRequest, Message: "unknown archive format", Detail: format}
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
		format, ok := parseArchiveFormat(in.Format)
		if !ok {
			return errorResult("archive", "compress", unknownFormatError(in.Format)), nil, nil
		}
		res := drainProgress(ctx, "archive", "compress", func(emit func(tools.Progress) error) error {
			return h.Archive.Compress(ctx, server.CompressParams{
				Sources:          in.Sources,
				Format:           format,
				Output:           in.Output,
				Password:         in.Password,
				CompressionLevel: in.CompressionLevel,
				Overwrite:        in.Overwrite,
			}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "archive_pack_inspect",
		Description: "Dry-run an archive pack: validate sources, probe output directory writability, return preflight Issues without writing the archive.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in archiveCompressInput) (*mcp.CallToolResult, any, error) {
		format, ok := parseArchiveFormat(in.Format)
		if !ok {
			return errorResult("archive", "pack_inspect", unknownFormatError(in.Format)), nil, nil
		}
		ins, err := h.Archive.InspectCompress(server.CompressParams{
			Sources: in.Sources,
			Format:  format,
			Output:  in.Output,
		})
		if err != nil {
			return errorResult("archive", "pack_inspect", err), nil, nil
		}
		out := map[string]any{
			"format":      ins.Format.String(),
			"entry_count": ins.EntryCount,
		}
		if len(ins.Issues) > 0 {
			out["issues"] = ins.Issues
		}
		return jsonResult("archive.pack_inspect: ok\n", out), nil, nil
	})
}
