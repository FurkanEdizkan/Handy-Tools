package http

import (
	"net/http"

	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

func (s *Server) handleSysdep(w http.ResponseWriter, _ *http.Request) {
	results := sysdep.All()
	out := make([]sysdepResult, 0, len(results))
	for _, r := range results {
		out = append(out, sysdepResult{
			Name:        r.Tool.Name,
			Found:       r.Found,
			Path:        r.Path,
			UsedAlias:   r.UsedAlias,
			Description: r.Tool.Description,
			Features:    r.Tool.Features,
			InstallHint: r.Tool.InstallHint,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
