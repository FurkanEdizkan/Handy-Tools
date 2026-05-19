package http

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/server"
)

// Server is the HTTP/SSE transport. It owns its own *http.Server and shares
// the Image/Archive/PDF handlers with the gRPC server — both transports drive
// the same internal/tools code through internal/server.{Image,Archive,PDF}Handler.
//
// Lifecycle:
//
//	s := New(opts)
//	go s.Serve(listener)   // or .ListenAndServe(":8080")
//	...
//	s.Shutdown(ctx)
type Server struct {
	Opts server.Options

	Image   *server.ImageHandler
	Archive *server.ArchiveHandler
	PDF     *server.PDFHandler

	jobs *jobs
	mux  *http.ServeMux
	http *http.Server
}

// New builds a Server with handlers wired against the same options the gRPC
// transport uses. The caller passes a server.Options containing AllowRoots so
// path safety stays centralised in one place.
func New(opts server.Options) *Server {
	s := &Server{
		Opts:    opts,
		Image:   &server.ImageHandler{Opts: opts},
		Archive: &server.ArchiveHandler{Opts: opts},
		PDF:     &server.PDFHandler{Opts: opts},
		jobs:    newJobs(),
	}
	s.mux = s.routes()
	return s
}

// Handler exposes the underlying mux so tests can drive it via httptest.
func (s *Server) Handler() http.Handler { return s.mux }

// Serve runs the HTTP server on the given listener. It blocks until Shutdown
// is called or the listener returns an error.
func (s *Server) Serve(lis net.Listener) error {
	s.http = &http.Server{
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := s.http.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/image/convert", s.handleImageConvert)
	mux.HandleFunc("POST /v1/archive/inspect", s.handleArchiveInspect)
	mux.HandleFunc("POST /v1/archive/extract", s.handleArchiveExtract)
	mux.HandleFunc("POST /v1/pdf/to-image", s.handlePDFToImage)
	mux.HandleFunc("POST /v1/pdf/to-text", s.handlePDFToText)
	mux.HandleFunc("POST /v1/pdf/merge", s.handlePDFMerge)
	mux.HandleFunc("GET /v1/jobs/{id}/events", s.handleJobEvents)
	mux.HandleFunc("GET /v1/sysdep", s.handleSysdep)
	return mux
}

// decodeJSON parses the request body into v with a strict reader: extra fields
// are tolerated (forward-compat for frontends) but bodies larger than 1 MiB are
// refused so a misbehaving client can't OOM the server.
func decodeJSON(r *http.Request, v any) error {
	const limit = 1 << 20
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, limit))
	return dec.Decode(v)
}

// writeJSON serializes v with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
