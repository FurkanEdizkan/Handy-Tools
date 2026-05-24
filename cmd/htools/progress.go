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
	quiet bool
	json  bool
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
				Level:       severityName(ev.Level),
				Message:     ev.Message,
				Completed:   ev.Completed,
				ErrorCode:   errCode(ev.Err),
				ErrorDetail: errDetail(ev.Err),
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
		}
		return exitCode(last.Err)
	}
	if !opts.quiet && !opts.json && last.Message != "" {
		fmt.Fprintln(os.Stdout, last.Message)
	}
	return 0
}

// exitCode maps a structured tools.Error to a process exit code.
func exitCode(e *tools.Error) int {
	if e == nil {
		return 0
	}
	switch e.Code {
	case tools.CodeBadRequest, tools.CodeUnsupportedInput, tools.CodeMissingBinary:
		return 1
	case tools.CodeIO, tools.CodeAborted:
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
	Tool        string  `json:"tool,omitempty"`
	Action      string  `json:"action,omitempty"`
	Item        string  `json:"item,omitempty"`
	Fraction    float64 `json:"fraction,omitempty"`
	Level       string  `json:"level,omitempty"`
	Message     string  `json:"message,omitempty"`
	Completed   bool    `json:"completed,omitempty"`
	ErrorCode   string  `json:"error_code,omitempty"`
	ErrorDetail string  `json:"error_detail,omitempty"`
}

// usageErr reports a flag-parsing problem and returns exit code 1.
func usageErr(w io.Writer, verb, msg string) int {
	fmt.Fprintf(w, "%s: %s\n", verb, msg)
	return 1
}
