package http

import (
	"net/http"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/buildinfo"
	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// handleHealth reports server liveness for the web connection-status badge
// (#14). It is intentionally unauthenticated and does no filesystem access —
// no Options.CheckPath — so a client can probe it before doing anything else.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	available := []string{}
	for _, r := range sysdep.All() {
		if r.Found {
			available = append(available, r.Tool.Name)
		}
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Version:        buildinfo.Version,
		UptimeSeconds:  int64(time.Since(s.startedAt).Seconds()),
		Transports:     []string{"grpc", "http"},
		ToolsAvailable: available,
	})
}
