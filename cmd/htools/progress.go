package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// progressOpts bundles the cross-cutting output flags so each subcommand
// doesn't reimplement quiet/json switching.
type progressOpts struct {
	quiet  bool
	json   bool
	strict bool
}

// runPreflight renders Inspect-time issues (missing/unreadable inputs,
// unwritable outputs) and returns exit code 2 if --strict and any issue is
// present. Issues render to stderr in text mode and as one JSON object per
// issue (kind: "preflight") on stdout in --json mode. Returns 0 when there
// are no issues or when --strict is false.
func runPreflight(issues []tools.PathIssue, opts progressOpts) int {
	if len(issues) == 0 {
		return 0
	}
	if opts.json {
		enc := json.NewEncoder(os.Stdout)
		for _, iss := range issues {
			_ = enc.Encode(preflightJSON{
				Kind:   "preflight",
				Path:   iss.Path,
				Code:   iss.Code,
				Detail: iss.Detail,
			})
		}
	} else if !opts.quiet {
		for _, iss := range issues {
			fmt.Fprintf(os.Stderr, "preflight: %s: %s (%s)\n", iss.Path, iss.Code, iss.Detail)
		}
	}
	if opts.strict {
		return 2
	}
	return 0
}

type preflightJSON struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// streamProgress drains a tool's progress channel to stdout/stderr and
// returns an exit code derived from the terminal event's error code. Each
// non-terminal event becomes one human line on stderr (or one JSON line on
// stdout when --json is set); the terminal event's Message is written to
// stdout for piping.
func streamProgress(ch <-chan tools.Progress, opts progressOpts) int {
	var last tools.Progress
	enc := json.NewEncoder(os.Stdout)
	for ev := range ch {
		last = ev
		if opts.json {
			_ = enc.Encode(progressJSON{
				Tool:        ev.Tool,
				Action:      ev.Action,
				Item:        ev.CurrentItem,
				Fraction:    ev.Fraction,
				Estimated:   ev.Estimated,
				Level:       severityName(ev.Level),
				Message:     ev.Message,
				Completed:   ev.Completed,
				ErrorCode:   errCode(ev.Err),
				ErrorDetail: errDetail(ev.Err),
				Failures:    failuresJSON(ev.Failures),
			})
			continue
		}
		if opts.quiet {
			continue
		}
		if ev.Completed {
			continue // terminal handled below
		}
		if ev.Message != "" {
			fmt.Fprintln(os.Stderr, ev.Message)
		}
	}
	return finalize(last, opts)
}

// finalize writes the terminal event's message and returns the exit code.
func finalize(last tools.Progress, opts progressOpts) int {
	if last.Err != nil {
		if !opts.json {
			fmt.Fprintf(os.Stderr, "error: %s\n", last.Err.Error())
			renderFailures(last.Failures, opts)
		}
		return exitCode(last.Err)
	}
	if !opts.quiet && !opts.json {
		if last.Message != "" {
			fmt.Fprintln(os.Stdout, last.Message)
		}
		renderFailures(last.Failures, opts)
	}
	return 0
}

// renderFailures prints one line per per-file failure to stderr in text
// mode. Skipped when --quiet or --json (JSON consumers see the failures
// field on the terminal progressJSON event).
func renderFailures(failures []tools.Failure, opts progressOpts) {
	if len(failures) == 0 || opts.quiet || opts.json {
		return
	}
	for _, f := range failures {
		fmt.Fprintf(os.Stderr, "  %s: %s %s\n", f.Path, f.Code, f.Message)
	}
}

// failuresJSON converts the in-package Failure slice to the JSON-wire shape.
func failuresJSON(failures []tools.Failure) []failureJSON {
	if len(failures) == 0 {
		return nil
	}
	out := make([]failureJSON, len(failures))
	for i, f := range failures {
		out[i] = failureJSON{Path: f.Path, Code: f.Code, Message: f.Message}
	}
	return out
}

// exitCode maps a structured tools.Error to a process exit code.
func exitCode(e *tools.Error) int {
	if e == nil {
		return 0
	}
	switch e.Code {
	case tools.CodeBadRequest, tools.CodeUnsupportedInput, tools.CodeMissingBinary:
		return 1
	case tools.CodeIO, tools.CodePermissionDenied, tools.CodeNotFound, tools.CodeAborted:
		return 2
	}
	return 2
}

func errCode(e *tools.Error) string {
	if e == nil {
		return ""
	}
	return e.Code
}

func errDetail(e *tools.Error) string {
	if e == nil {
		return ""
	}
	return e.Detail
}

func severityName(s tools.Severity) string {
	switch s {
	case tools.SeverityInfo:
		return "info"
	case tools.SeverityWarning:
		return "warning"
	case tools.SeverityError:
		return "error"
	}
	return ""
}

type progressJSON struct {
	Tool        string        `json:"tool,omitempty"`
	Action      string        `json:"action,omitempty"`
	Item        string        `json:"item,omitempty"`
	Fraction    float64       `json:"fraction,omitempty"`
	Estimated   bool          `json:"estimated,omitempty"`
	Level       string        `json:"level,omitempty"`
	Message     string        `json:"message,omitempty"`
	Completed   bool          `json:"completed,omitempty"`
	ErrorCode   string        `json:"error_code,omitempty"`
	ErrorDetail string        `json:"error_detail,omitempty"`
	Failures    []failureJSON `json:"failures,omitempty"`
}

type failureJSON struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// usageErr reports a flag-parsing problem and returns exit code 1.
func usageErr(w io.Writer, verb, msg string) int {
	fmt.Fprintf(w, "%s: %s\n", verb, msg)
	return 1
}
