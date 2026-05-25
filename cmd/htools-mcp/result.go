package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// runResult is the structured payload every tool's MCP response carries
// alongside the human-readable text summary. The "any" output slot of the
// SDK's typed handler picks this up and serialises it as the result's
// structuredContent, so an LLM can read either the prose or the JSON.
type runResult struct {
	Tool      string   `json:"tool"`
	Action    string   `json:"action"`
	OK        bool     `json:"ok"`
	Messages  []string `json:"messages,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	ErrorCode string   `json:"error_code,omitempty"`
	ErrorMsg  string   `json:"error_message,omitempty"`
	ErrorHint string   `json:"error_hint,omitempty"`
}

// drainProgress invokes fn(emit) and accumulates every tools.Progress event.
// The terminal event's success/failure shapes the returned runResult. If a
// non-progress error returns from fn itself (e.g. path validation), it is
// reported as the result's error block — the caller still gets a
// CallToolResult so the LLM sees a structured response instead of a raw
// protocol error.
func drainProgress(ctx context.Context, toolName, action string, fn func(emit func(tools.Progress) error) error) *mcp.CallToolResult {
	res := &runResult{Tool: toolName, Action: action}
	if err := ctx.Err(); err != nil {
		res.ErrorCode = tools.CodeAborted
		res.ErrorMsg = err.Error()
		return toResult(res)
	}
	var lines []string
	emit := func(p tools.Progress) error {
		if msg := strings.TrimSpace(p.Message); msg != "" {
			lines = append(lines, msg)
		}
		if p.Completed {
			if p.Err != nil {
				res.ErrorCode = p.Err.Code
				res.ErrorMsg = p.Err.Message
				res.ErrorHint = p.Err.Detail
			} else {
				res.OK = true
				res.Summary = strings.TrimSpace(p.Message)
			}
		}
		return nil
	}
	if err := fn(emit); err != nil {
		// fn returned before producing a terminal Progress (typically a path
		// CheckPath rejection or a context error). Surface it the same way a
		// terminal Err would render.
		res.OK = false
		var terr *tools.Error
		if errors.As(err, &terr) {
			res.ErrorCode = terr.Code
			res.ErrorMsg = terr.Message
			res.ErrorHint = terr.Detail
		} else {
			res.ErrorCode = tools.CodeBadRequest
			res.ErrorMsg = err.Error()
		}
	}
	res.Messages = lines
	return toResult(res)
}

// toResult renders runResult as an MCP CallToolResult, attaching the
// runResult itself as StructuredContent so MCP clients can read either the
// human-readable prose or the structured payload.
func toResult(r *runResult) *mcp.CallToolResult {
	var b strings.Builder
	if r.OK {
		fmt.Fprintf(&b, "%s.%s: ok\n", r.Tool, r.Action)
	} else {
		fmt.Fprintf(&b, "%s.%s: failed (%s) %s\n", r.Tool, r.Action, r.ErrorCode, r.ErrorMsg)
		if r.ErrorHint != "" {
			fmt.Fprintf(&b, "hint: %s\n", r.ErrorHint)
		}
	}
	if r.Summary != "" && r.Summary != r.ErrorMsg {
		fmt.Fprintf(&b, "summary: %s\n", r.Summary)
	}
	if n := len(r.Messages); n > 0 {
		fmt.Fprintf(&b, "\nprogress (%d events):\n", n)
		for _, m := range r.Messages {
			fmt.Fprintf(&b, "  - %s\n", m)
		}
	}
	return &mcp.CallToolResult{
		IsError:           !r.OK,
		Content:           []mcp.Content{&mcp.TextContent{Text: b.String()}},
		StructuredContent: r,
	}
}

// jsonResult builds an MCP result whose body is a single text block containing
// a short human-readable header plus the structured payload. Used for the
// sync-only tools (archive_inspect, rename_inspect, diff_tree_inspect,
// hash_verify, doctor) that don't go through the progress drain.
func jsonResult(header string, structured any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: header}},
		StructuredContent: structured,
	}
}

// errorResult is the short-circuit for the sync-only tools when path
// validation or another preflight fails before the tool package is reached.
func errorResult(toolName, action string, err error) *mcp.CallToolResult {
	r := &runResult{Tool: toolName, Action: action}
	var terr *tools.Error
	if errors.As(err, &terr) {
		r.ErrorCode = terr.Code
		r.ErrorMsg = terr.Message
		r.ErrorHint = terr.Detail
	} else {
		r.ErrorCode = tools.CodeBadRequest
		r.ErrorMsg = err.Error()
	}
	return toResult(r)
}
