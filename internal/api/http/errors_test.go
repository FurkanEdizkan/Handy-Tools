package http

import (
	"net/http"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

func TestStatusForCode(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{tools.CodeBadRequest, http.StatusBadRequest},
		{tools.CodeUnsupportedInput, http.StatusUnsupportedMediaType},
		{tools.CodeMissingBinary, http.StatusServiceUnavailable},
		{tools.CodePermissionDenied, http.StatusForbidden},
		{tools.CodeNotFound, http.StatusNotFound},
		{tools.CodeAborted, 499},
		{tools.CodeIO, http.StatusInternalServerError},
		{"UNKNOWN_FUTURE_CODE", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			if got := statusForCode(tc.code); got != tc.want {
				t.Fatalf("statusForCode(%q) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}
