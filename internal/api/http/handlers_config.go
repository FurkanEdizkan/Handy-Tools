package http

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// handleConfigGet returns the full user config as JSON.
func (s *Server) handleConfigGet(w http.ResponseWriter, _ *http.Request) {
	cfg, _, err := config.Load()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleConfigPatch deep-merges a partial Config from the request body into the
// current config, validates it, and persists it. Writes to `server.*` are
// rejected — it is a startup-time security boundary, not a user setting.
func (s *Server) handleConfigPatch(w http.ResponseWriter, r *http.Request) {
	const limit = 1 << 20
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		writeError(w, err)
		return
	}

	// Inspect the patch's top-level keys before merging.
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(body, &keys); err != nil {
		writeError(w, &tools.Error{
			Code: tools.CodeBadRequest, Message: "invalid JSON body", Detail: err.Error(),
		})
		return
	}
	if _, ok := keys["server"]; ok {
		writeError(w, &tools.Error{
			Code:    tools.CodeBadRequest,
			Message: "server.* is a startup-time setting and cannot be changed via the API",
		})
		return
	}

	cfg, _, err := config.Load()
	if err != nil {
		writeError(w, err)
		return
	}

	// Unmarshalling the patch onto the existing struct leaves unmentioned
	// fields untouched, recursively — i.e. a deep merge.
	if err := json.Unmarshal(body, &cfg); err != nil {
		writeError(w, &tools.Error{
			Code: tools.CodeBadRequest, Message: "invalid config patch", Detail: err.Error(),
		})
		return
	}

	if err := validateConfig(cfg); err != nil {
		writeError(w, err)
		return
	}

	// If the patch set a filesystem path, it must stay inside the allow-roots.
	if archiveRaw, ok := keys["archive"]; ok {
		var archiveKeys map[string]json.RawMessage
		if json.Unmarshal(archiveRaw, &archiveKeys) == nil {
			if _, hasDest := archiveKeys["default_destination"]; hasDest && cfg.Archive.DefaultDestination != "" {
				if _, err := s.Opts.CheckPath(cfg.Archive.DefaultDestination); err != nil {
					writeError(w, err)
					return
				}
			}
		}
	}

	if _, err := config.Save(cfg); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// validateConfig enforces the invariants a persisted config must satisfy.
func validateConfig(c config.Config) error {
	if c.Image.DefaultJPEGQuality < 1 || c.Image.DefaultJPEGQuality > 100 {
		return &tools.Error{
			Code: tools.CodeBadRequest, Message: "image.default_jpeg_quality must be in [1,100]",
		}
	}
	if c.PDF.DefaultDPI <= 0 {
		return &tools.Error{
			Code: tools.CodeBadRequest, Message: "pdf.default_dpi must be greater than 0",
		}
	}
	return nil
}
