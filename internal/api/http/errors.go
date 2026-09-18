package http

import (
	"encoding/json"
	"net/http"

	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// errorEnvelope is the JSON body of every non-2xx response.
type errorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// writeError serializes err onto w with a status derived from the tool error
// code. Every error is normalized through server.ToolError first, so the
// client always sees the same envelope shape and the same code the gRPC and
// MCP transports report.
func writeError(w http.ResponseWriter, err error) {
	status, env := classify(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]errorEnvelope{"error": env})
}

// classify maps err onto an HTTP status and envelope. *tools.Error values
// (including CheckPath rejections, which are BAD_REQUEST) keep their code;
// plain filesystem errors become NOT_FOUND / PERMISSION_DENIED; anything
// else (malformed JSON, oversized body, ...) is BAD_REQUEST.
func classify(err error) (int, errorEnvelope) {
	te := server.ToolError(err)
	return statusForCode(te.Code), errorEnvelope{
		Code:    te.Code,
		Message: te.Message,
		Detail:  te.Detail,
	}
}

func statusForCode(code string) int {
	switch code {
	case tools.CodeBadRequest:
		return http.StatusBadRequest
	case tools.CodeUnsupportedInput:
		return http.StatusUnsupportedMediaType
	case tools.CodeMissingBinary:
		return http.StatusServiceUnavailable
	case tools.CodePermissionDenied:
		return http.StatusForbidden
	case tools.CodeNotFound:
		return http.StatusNotFound
	case tools.CodeAborted:
		// Mirrors the nginx "client closed request" semantics: the work
		// stopped due to a caller-side action (timeout / cancel) and the
		// HTTP code reflects that.
		return 499
	case tools.CodeIO:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// toolErrorFromProgress lifts a *tools.Error from a Progress event into the
// envelope shape used inside the SSE data: frames.
func toolErrorFromProgress(e *tools.Error) *errorEnvelope {
	if e == nil {
		return nil
	}
	return &errorEnvelope{Code: e.Code, Message: e.Message, Detail: e.Detail}
}
