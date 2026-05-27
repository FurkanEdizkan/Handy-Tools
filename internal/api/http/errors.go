package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// errorEnvelope is the JSON body of every non-2xx response.
type errorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// writeError serializes err onto w with a status derived from the tool error
// code. Non-tool errors (e.g. CheckPath rejections) become 400 BAD_REQUEST so
// the client always sees the same envelope shape.
func writeError(w http.ResponseWriter, err error) {
	status, env := classify(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]errorEnvelope{"error": env})
}

func classify(err error) (int, errorEnvelope) {
	var te *tools.Error
	if errors.As(err, &te) {
		return statusForCode(te.Code), errorEnvelope{
			Code:    te.Code,
			Message: te.Message,
			Detail:  te.Detail,
		}
	}
	// Plain Go errors — most often CheckPath rejections — collapse to
	// BAD_REQUEST so the client sees one consistent envelope.
	return http.StatusBadRequest, errorEnvelope{
		Code:    tools.CodeBadRequest,
		Message: err.Error(),
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
