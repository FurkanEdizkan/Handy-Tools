package http

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// progressToWire converts an in-process Progress event into the JSON-friendly
// wire shape sent inside SSE data: frames.
func progressToWire(p tools.Progress) progressEvent {
	out := progressEvent{
		JobID:       p.JobID,
		Tool:        p.Tool,
		Action:      p.Action,
		CurrentItem: p.CurrentItem,
		BytesDone:   p.BytesDone,
		BytesTotal:  p.BytesTotal,
		Fraction:    p.Fraction,
		Level:       severityName(p.Level),
		Message:     p.Message,
		Completed:   p.Completed,
		Error:       toolErrorFromProgress(p.Err),
	}
	if !p.StartedAt.IsZero() {
		out.StartedAt = p.StartedAt.UnixMilli()
	}
	return out
}

func severityName(s tools.Severity) string {
	switch s {
	case tools.SeverityInfo:
		return "INFO"
	case tools.SeverityWarning:
		return "WARNING"
	case tools.SeverityError:
		return "ERROR"
	}
	return ""
}

// writeSSEEvent writes one Server-Sent-Events frame: a "data: <json>\n\n"
// block followed by a flush. The flusher is required because intermediaries
// will otherwise buffer until the response body closes.
func writeSSEEvent(w io.Writer, flush func(), p tools.Progress) error {
	buf, err := json.Marshal(progressToWire(p))
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", buf); err != nil {
		return err
	}
	if flush != nil {
		flush()
	}
	return nil
}
