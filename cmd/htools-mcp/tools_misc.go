package main

import (
	"context"
	"runtime"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

type hashRunInput struct {
	Sources []string `json:"sources" jsonschema:"absolute paths of files to hash"`
	Algo    string   `json:"algo,omitempty" jsonschema:"one of: md5, sha256 (default), blake3"`
}

type hashVerifyInput struct {
	Manifest string `json:"manifest" jsonschema:"absolute path of a sha256sum-format manifest (DIGEST␣␣PATH per line)"`
	Algo     string `json:"algo,omitempty" jsonschema:"one of: md5, sha256 (default), blake3"`
}

type diffTreeInput struct {
	A    string `json:"a" jsonschema:"absolute path of the first directory tree"`
	B    string `json:"b" jsonschema:"absolute path of the second directory tree"`
	Mode string `json:"mode,omitempty" jsonschema:"comparison mode: mtime (default, fast) or hash (slow, authoritative)"`
}

type renameInspectInput struct {
	Sources     []string `json:"sources" jsonschema:"absolute paths to consider for renaming"`
	Pattern     string   `json:"pattern" jsonschema:"Go regex matched against each basename"`
	Replace     string   `json:"replace" jsonschema:"replacement template (regex backrefs allowed)"`
	OnCollision string   `json:"on_collision,omitempty" jsonschema:"how to handle target collisions: error (default), skip, suffix"`
}

type renameRunInput = renameInspectInput

type doctorInput struct{}

// doctorEntry is what the doctor tool reports for each known optional binary.
// Keeping the field set small so the LLM can scan it.
type doctorEntry struct {
	Name        string   `json:"name"`
	Found       bool     `json:"found"`
	Path        string   `json:"path,omitempty"`
	UsedAlias   string   `json:"used_alias,omitempty"`
	Features    []string `json:"features"`
	InstallHint string   `json:"install_hint,omitempty"`
}

func registerMiscTools(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hash",
		Description: "Compute MD5/SHA256/BLAKE3 digests of one or more files; the per-file message is in sha256sum format (DIGEST␣␣PATH).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in hashRunInput) (*mcp.CallToolResult, any, error) {
		algo := in.Algo
		if algo == "" {
			algo = "sha256"
		}
		res := drainProgress(ctx, "hash", "run", func(emit func(tools.Progress) error) error {
			return h.Hash.Run(ctx, server.HashRunParams{
				Sources: in.Sources,
				Algo:    algo,
			}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "hash_verify",
		Description: "Verify a sha256sum-format manifest: recomputes each entry's digest and reports OK / failed / missing counts plus per-row detail.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in hashVerifyInput) (*mcp.CallToolResult, any, error) {
		algo := in.Algo
		if algo == "" {
			algo = "sha256"
		}
		out, err := h.Hash.Verify(ctx, server.HashVerifyParams{Manifest: in.Manifest, Algo: algo})
		if err != nil {
			return errorResult("hash", "verify", err), nil, nil
		}
		return jsonResult("hash.verify: ok\n", out), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "diff_tree",
		Description: "Compare two directory trees and report added / removed / changed paths. Mode 'mtime' is fast; 'hash' reads every byte but is authoritative.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in diffTreeInput) (*mcp.CallToolResult, any, error) {
		mode := in.Mode
		if mode == "" {
			mode = "mtime"
		}
		res := drainProgress(ctx, "diff_tree", "run", func(emit func(tools.Progress) error) error {
			return h.DiffTree.Run(ctx, server.DiffTreeParams{A: in.A, B: in.B, Mode: mode}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rename_inspect",
		Description: "Dry-run a regex-based rename batch and return the plan (no filesystem mutation).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in renameInspectInput) (*mcp.CallToolResult, any, error) {
		plans, err := h.Rename.Inspect(server.RenameParams{
			Sources:     in.Sources,
			Pattern:     in.Pattern,
			Replace:     in.Replace,
			OnCollision: in.OnCollision,
		})
		if err != nil {
			return errorResult("rename", "inspect", err), nil, nil
		}
		// MCP spec: structuredContent must be a JSON object, not an array.
		// Wrap the slice so strict clients (e.g. the official Go SDK
		// validator) accept the response.
		return jsonResult("rename.inspect: ok\n", map[string]any{"plans": plans}), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rename_run",
		Description: "Execute a regex-based rename batch. on_collision: error (abort), skip, or suffix (-1, -2, ...).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in renameRunInput) (*mcp.CallToolResult, any, error) {
		res := drainProgress(ctx, "rename", "run", func(emit func(tools.Progress) error) error {
			return h.Rename.Run(ctx, server.RenameParams{
				Sources:     in.Sources,
				Pattern:     in.Pattern,
				Replace:     in.Replace,
				OnCollision: in.OnCollision,
			}, emit)
		})
		return res, nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "doctor",
		Description: "Report which optional system binaries (unrar, 7z, pdftoppm, pdftotext, magick) are available on PATH and what features they unlock.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ doctorInput) (*mcp.CallToolResult, any, error) {
		results := sysdep.All()
		entries := make([]doctorEntry, 0, len(results))
		for _, r := range results {
			entries = append(entries, doctorEntry{
				Name:        r.Tool.Name,
				Found:       r.Found,
				Path:        r.Path,
				UsedAlias:   r.UsedAlias,
				Features:    r.Tool.Features,
				InstallHint: r.Tool.InstallHint[runtime.GOOS],
			})
		}
		// See rename_inspect above — structuredContent must be an object.
		return jsonResult("doctor: ok\n", map[string]any{"tools": entries}), nil, nil
	})
}
