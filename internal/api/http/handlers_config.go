package http

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"

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
	if err := requireJSON(r); err != nil {
		writeError(w, err)
		return
	}
	const limit = 1 << 20
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		writeError(w, err)
		return
	}

	// Inspect the patch's top-level keys before merging. encoding/json
	// matches object keys to struct fields case-insensitively (with Unicode
	// folding: "Server", "SERVER", "ſerver" all hit Config.Server), so every
	// key check here uses strings.EqualFold — the same folding — rather than
	// an exact map lookup.
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(body, &keys); err != nil {
		writeError(w, &tools.Error{
			Code: tools.CodeBadRequest, Message: "invalid JSON body", Detail: err.Error(),
		})
		return
	}
	if hasKeyFold(keys, "server") {
		writeError(w, errServerReadOnly)
		return
	}

	cfg, _, err := config.Load()
	if err != nil {
		writeError(w, err)
		return
	}
	before := cfg

	// Unmarshalling the patch onto the existing struct leaves unmentioned
	// fields untouched, recursively — i.e. a deep merge.
	if err := json.Unmarshal(body, &cfg); err != nil {
		writeError(w, &tools.Error{
			Code: tools.CodeBadRequest, Message: "invalid config patch", Detail: err.Error(),
		})
		return
	}

	// Belt and braces: whatever key spelling reached it, the merge must not
	// have altered server.*.
	if !reflect.DeepEqual(before.Server, cfg.Server) {
		writeError(w, errServerReadOnly)
		return
	}

	if err := validateConfig(cfg); err != nil {
		writeError(w, err)
		return
	}

	// If the patch set (or changed) a filesystem path, it must stay inside
	// the allow-roots.
	dest := cfg.Archive.DefaultDestination
	if dest != "" && (dest != before.Archive.DefaultDestination || patchSetsDestination(keys)) {
		if _, err := s.Opts.CheckPath(dest); err != nil {
			writeError(w, err)
			return
		}
	}

	if _, err := config.Save(cfg); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// errServerReadOnly rejects any PATCH that touches server.*.
var errServerReadOnly = &tools.Error{
	Code:    tools.CodeBadRequest,
	Message: "server.* is a startup-time setting and cannot be changed via the API",
}

// hasKeyFold reports whether m has key under the same case folding
// encoding/json uses to match object keys to struct fields.
func hasKeyFold(m map[string]json.RawMessage, key string) bool {
	for k := range m {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

// patchSetsDestination reports whether the patch mentions
// archive.default_destination under any key spelling.
func patchSetsDestination(keys map[string]json.RawMessage) bool {
	for k, raw := range keys {
		if !strings.EqualFold(k, "archive") {
			continue
		}
		var archiveKeys map[string]json.RawMessage
		if json.Unmarshal(raw, &archiveKeys) != nil {
			continue
		}
		if hasKeyFold(archiveKeys, "default_destination") {
			return true
		}
	}
	return false
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
