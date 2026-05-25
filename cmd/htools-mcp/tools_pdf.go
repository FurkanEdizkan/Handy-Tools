package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/pdf"
)

// pdfMergeInput is the MCP-visible shape for pdf_merge.
type pdfMergeInput struct {
	Sources    []string `json:"sources" jsonschema:"absolute paths of two or more PDFs to concatenate"`
	OutputFile string   `json:"output_file" jsonschema:"absolute path of the merged PDF to write"`
}

type pdfSplitInput struct {
	Source    string `json:"source" jsonschema:"absolute path of the PDF to split"`
	OutputDir string `json:"output_dir" jsonschema:"absolute directory the per-chunk PDFs are written to"`
	From      int    `json:"from,omitempty" jsonschema:"1-based first page of one explicit range; use 0 with every_n for chunk mode"`
	To        int    `json:"to,omitempty" jsonschema:"1-based last page (inclusive) of the explicit range; 0 means last page"`
	EveryN    int    `json:"every_n,omitempty" jsonschema:"chunk every N pages instead of an explicit range; mutually exclusive with from/to"`
}

type pdfRenderInput struct {
	Source    string `json:"source" jsonschema:"absolute path of the PDF to rasterise"`
	OutputDir string `json:"output_dir" jsonschema:"absolute directory the page images are written to"`
	From      int    `json:"from,omitempty" jsonschema:"1-based first page; 0 = start"`
	To        int    `json:"to,omitempty" jsonschema:"1-based last page (inclusive); 0 = end"`
	DPI       int    `json:"dpi,omitempty" jsonschema:"render DPI; 0 lets the tool pick a sensible default (~150)"`
	JPEG      bool   `json:"jpeg,omitempty" jsonschema:"emit JPEG instead of the PNG default"`
}

type pdfTextInput struct {
	Source     string `json:"source" jsonschema:"absolute path of the PDF to extract text from"`
	OutputFile string `json:"output_file,omitempty" jsonschema:"absolute path of the .txt to write; empty defaults to {source}.txt"`
	From       int    `json:"from,omitempty" jsonschema:"1-based first page; 0 = start"`
	To         int    `json:"to,omitempty" jsonschema:"1-based last page (inclusive); 0 = end"`
	Layout     bool   `json:"layout,omitempty" jsonschema:"preserve original column layout (pdftotext -layout)"`
}

func registerPDFTools(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pdf_merge",
		Description: "Concatenate two or more PDFs into a single output PDF (pure-Go, no external binary).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in pdfMergeInput) (*mcp.CallToolResult, any, error) {
		res := drainProgress(ctx, "pdf", "merge", func(emit func(tools.Progress) error) error {
			return h.PDF.Merge(ctx, server.PDFMergeParams{
				Sources:    in.Sources,
				OutputFile: in.OutputFile,
			}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pdf_split",
		Description: "Split a PDF either by an explicit page range (from/to) or every N pages (pure-Go).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in pdfSplitInput) (*mcp.CallToolResult, any, error) {
		var ranges []pdf.Range
		if in.From > 0 || in.To > 0 {
			ranges = []pdf.Range{{From: in.From, To: in.To}}
		}
		res := drainProgress(ctx, "pdf", "split", func(emit func(tools.Progress) error) error {
			return h.PDF.Split(ctx, server.PDFSplitParams{
				Source:     in.Source,
				PageRanges: ranges,
				EveryN:     in.EveryN,
				OutputDir:  in.OutputDir,
			}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pdf_render",
		Description: "Rasterise PDF pages to PNG (default) or JPEG. Requires the pdftoppm binary (poppler-utils).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in pdfRenderInput) (*mcp.CallToolResult, any, error) {
		res := drainProgress(ctx, "pdf", "render", func(emit func(tools.Progress) error) error {
			return h.PDF.ToImage(ctx, server.PDFToImageParams{
				Source:    in.Source,
				From:      in.From,
				To:        in.To,
				DPI:       in.DPI,
				OutputDir: in.OutputDir,
				JPEG:      in.JPEG,
			}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pdf_text",
		Description: "Extract plain text from a PDF. Requires the pdftotext binary (poppler-utils).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in pdfTextInput) (*mcp.CallToolResult, any, error) {
		res := drainProgress(ctx, "pdf", "text", func(emit func(tools.Progress) error) error {
			return h.PDF.ToText(ctx, server.PDFToTextParams{
				Source:     in.Source,
				From:       in.From,
				To:         in.To,
				Layout:     in.Layout,
				OutputFile: in.OutputFile,
			}, emit)
		})
		return res, nil, nil
	})
}
