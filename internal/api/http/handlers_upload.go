package http

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// handleUploadCreate stages a multipart/form-data upload into a fresh temp
// workspace. Each "files" part is written into the workspace in/ directory;
// the response hands the browser back server-side paths it can feed straight
// into the existing tool endpoints, plus the workspace out/ directory to use
// as the tool output location. The whole request body is capped at the
// Manager's MaxBytes via http.MaxBytesReader.
func (s *Server) handleUploadCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.uploads.MaxBytes)

	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, &tools.Error{
			Code:    tools.CodeBadRequest,
			Message: "expected multipart/form-data body",
			Detail:  err.Error(),
		})
		return
	}

	ws, err := s.uploads.Create()
	if err != nil {
		writeError(w, &tools.Error{Code: tools.CodeIO, Message: "create workspace", Detail: err.Error()})
		return
	}

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = s.uploads.Remove(ws.ID)
			writeUploadReadError(w, err)
			return
		}
		if part.FormName() != "files" || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		if _, err := s.uploads.Save(ws, part.FileName(), part); err != nil {
			_ = part.Close()
			_ = s.uploads.Remove(ws.ID)
			writeUploadReadError(w, err)
			return
		}
		_ = part.Close()
	}

	if len(ws.Files) == 0 {
		_ = s.uploads.Remove(ws.ID)
		writeError(w, &tools.Error{
			Code:    tools.CodeBadRequest,
			Message: "no files in upload",
			Detail:  `expected one or more "files" form parts`,
		})
		return
	}

	resp := uploadCreateResponse{UploadID: ws.ID, OutputDir: ws.OutDir}
	for _, f := range ws.Files {
		resp.Files = append(resp.Files, uploadedFile{Name: f.Name, Path: f.Path})
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleUploadDownload streams the result a tool wrote into a workspace's out/
// directory. A single output file is streamed verbatim with a download
// disposition; multiple files (e.g. an archive extraction or a multi-page PDF
// render) are bundled into a zip on the fly.
func (s *Server) handleUploadDownload(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.uploads.Get(r.PathValue("id"))
	if !ok {
		writeError(w, &tools.Error{Code: tools.CodeBadRequest, Message: "unknown upload id"})
		return
	}

	files, err := outputFiles(ws.OutDir)
	if err != nil {
		writeError(w, &tools.Error{Code: tools.CodeIO, Message: "read output", Detail: err.Error()})
		return
	}
	switch len(files) {
	case 0:
		writeError(w, &tools.Error{
			Code:    tools.CodeBadRequest,
			Message: "no output produced yet",
			Detail:  "run a tool against this upload before downloading",
		})
	case 1:
		streamFile(w, files[0])
	default:
		streamZip(w, files, ws.OutDir)
	}
}

// handleUploadDelete reaps a workspace on explicit client request. Deleting an
// unknown id is a no-op so the call is safe to fire on page unload.
func (s *Server) handleUploadDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.uploads.Remove(r.PathValue("id")); err != nil {
		writeError(w, &tools.Error{Code: tools.CodeIO, Message: "remove workspace", Detail: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeUploadReadError maps a body-read failure to its envelope. An oversized
// body (http.MaxBytesReader tripped) becomes 413; anything else is a 400.
func writeUploadReadError(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_ = json.NewEncoder(w).Encode(map[string]errorEnvelope{"error": {
			Code:    tools.CodeBadRequest,
			Message: "upload exceeds size limit",
		}})
		return
	}
	writeError(w, &tools.Error{Code: tools.CodeBadRequest, Message: "read upload", Detail: err.Error()})
}

// outputFiles lists every regular file under dir, recursively. Paths are
// returned absolute; an empty slice means the tool has written nothing yet.
// dir is always a workspace out/ directory — a server-owned path with a
// crypto-random id component, never caller-supplied.
func outputFiles(dir string) ([]string, error) {
	var out []string
	//nolint:gosec // dir is a server-owned upload workspace path, not user input
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// streamFile sends a single output file with an attachment disposition.
func streamFile(w http.ResponseWriter, path string) {
	f, err := os.Open(path) //nolint:gosec // path comes from a server-owned workspace walk
	if err != nil {
		writeError(w, &tools.Error{Code: tools.CodeIO, Message: "open output", Detail: err.Error()})
		return
	}
	defer f.Close()

	name := filepath.Base(path)
	if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", attachment(name))
	_, _ = io.Copy(w, f)
}

// streamZip bundles multiple output files into a zip, preserving their paths
// relative to the workspace out/ directory.
func streamZip(w http.ResponseWriter, files []string, root string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", attachment("result.zip"))

	zw := zip.NewWriter(w)
	defer zw.Close()
	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = filepath.Base(path)
		}
		entry, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return
		}
		f, err := os.Open(path) //nolint:gosec // path comes from a server-owned workspace walk
		if err != nil {
			return
		}
		_, copyErr := io.Copy(entry, f)
		_ = f.Close()
		if copyErr != nil {
			return
		}
	}
}

// attachment builds a Content-Disposition value, quoting the filename so a
// space or comma in it can't break the header.
func attachment(name string) string {
	return `attachment; filename="` + strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(name) + `"`
}
