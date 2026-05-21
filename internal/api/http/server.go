package http

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/tools"
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

	// Queue is the shared job registry — the same instance the gRPC server
	// uses — so a job started on either transport is visible to both.
	Queue *queue.Queue

	mux       *http.ServeMux
	http      *http.Server
	startedAt time.Time // captured in New(), reported by GET /v1/health
}

// New builds a Server with handlers wired against the same options the gRPC
// transport uses. The caller passes a server.Options containing AllowRoots so
// path safety stays centralised in one place, and the shared *queue.Queue so
// jobs are visible across both the HTTP and gRPC transports.
func New(opts server.Options, q *queue.Queue) *Server {
	s := &Server{
		Opts:      opts,
		Image:     &server.ImageHandler{Opts: opts},
		Archive:   &server.ArchiveHandler{Opts: opts},
		PDF:       &server.PDFHandler{Opts: opts},
		Queue:     q,
		startedAt: time.Now(),
	}
	s.mux = s.routes()
	if spa, err := newSPAHandler(); err == nil {
		s.mux.Handle("GET /", spa)
	}
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
	mux.HandleFunc("POST /v1/image/batch-convert", s.handleImageBatchConvert)
	mux.HandleFunc("POST /v1/archive/inspect", s.handleArchiveInspect)
	mux.HandleFunc("POST /v1/archive/extract", s.handleArchiveExtract)
	mux.HandleFunc("POST /v1/archive/compress", s.handleArchiveCompress)
	mux.HandleFunc("POST /v1/pdf/to-image", s.handlePDFToImage)
	mux.HandleFunc("POST /v1/pdf/to-text", s.handlePDFToText)
	mux.HandleFunc("POST /v1/pdf/merge", s.handlePDFMerge)
	mux.HandleFunc("POST /v1/pdf/split", s.handlePDFSplit)
	mux.HandleFunc("GET /v1/jobs/{id}/events", s.handleJobEvents)
	mux.HandleFunc("GET /v1/sysdep", s.handleSysdep)
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/config", s.handleConfigGet)
	mux.HandleFunc("PATCH /v1/config", s.handleConfigPatch)
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

// enqueue registers a tool job on the shared queue and writes the 202 envelope.
// run drives the tool with a timeout-bounded context and the queue's emit
// callback; a non-nil error from run is turned into a terminal failure
// Progress event so SSE subscribers see a structured failure. The queue runs
// the job in its own goroutine and marks it complete when run returns.
func (s *Server) enqueue(
	w http.ResponseWriter,
	tool, action string,
	timeout time.Duration,
	run func(ctx context.Context, emit func(tools.Progress)) error,
) {
	id := s.Queue.Enqueue(tool, action, func(ctx context.Context, emit func(tools.Progress)) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := run(ctx, emit); err != nil {
			emit(failureProgress(tool, action, err))
		}
	})
	writeJSON(w, http.StatusAccepted, jobResponse{JobID: id})
}
