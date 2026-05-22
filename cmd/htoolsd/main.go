// Command htoolsd is the Handy Tools server.
//
// It exposes the same tool packages the TUI uses (image, archive, pdf) over
// two transports:
//
//   - gRPC on --listen (default :7777)
//   - HTTP + Server-Sent Events on --http (default disabled)
//
// Both transports share the same Options.AllowRoots fail-closed sandbox, so
// the path-safety story is identical regardless of how a caller arrives.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpapi "github.com/furkandedizkan/handy-tools/internal/api/http"
	"github.com/furkandedizkan/handy-tools/internal/buildinfo"
	"github.com/furkandedizkan/handy-tools/internal/config"
	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/server"
)

func main() {
	listen := flag.String("listen", "", "gRPC address to listen on (overrides config)")
	httpListen := flag.String("http", "", "HTTP+SSE address to listen on (overrides config; empty = disabled)")
	allow := flag.String("allow-roots", "", "comma-separated allow-list of filesystem roots; overrides config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(buildinfo.String())
		return
	}

	cfg, _, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	grpcAddr := cfg.Server.Listen
	if *listen != "" {
		grpcAddr = *listen
	}
	httpAddr := cfg.Server.HTTPListen
	if *httpListen != "" {
		httpAddr = *httpListen
	}
	roots := cfg.Server.AllowRoots
	if *allow != "" {
		roots = splitCSV(*allow)
	}
	if grpcAddr == "" && httpAddr == "" {
		fmt.Fprintln(os.Stderr,
			"refusing to start: both gRPC (--listen) and HTTP (--http) are disabled.")
		os.Exit(2)
	}
	// Both transports run every FileRef path through Options.CheckPath, which
	// fails closed on an empty allow-list. Without roots there is nothing the
	// server can act on, so refuse to start rather than serve a dead surface.
	if len(roots) == 0 {
		fmt.Fprintln(os.Stderr,
			"refusing to start: no allow_roots configured. "+
				"Set --allow-roots or server.allow_roots.")
		os.Exit(2)
	}

	opts := server.Options{AllowRoots: roots}
	// One shared queue drives both transports, so a job started over gRPC is
	// visible to an HTTP/SSE subscriber and vice versa.
	q := queue.New()
	grpcSrv := server.New(opts, q)
	httpSrv := httpapi.New(opts, q)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 2)

	var grpcLis net.Listener
	if grpcAddr != "" {
		grpcLis, err = net.Listen("tcp", grpcAddr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "grpc listen:", err)
			os.Exit(1)
		}
		log.Printf("htoolsd gRPC listening on %s; roots=%v", grpcAddr, roots)
		go func() { errCh <- grpcSrv.Serve(grpcLis) }()
	}

	var httpLis net.Listener
	if httpAddr != "" {
		httpLis, err = net.Listen("tcp", httpAddr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "http listen:", err)
			os.Exit(1)
		}
		log.Printf("htoolsd HTTP listening on %s; roots=%v", httpAddr, roots)
		go func() { errCh <- httpSrv.Serve(httpLis) }()
	}

	go func() {
		<-stop
		log.Println("shutting down")
		grpcSrv.Stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	// Block on the first transport that returns an error (or nil on clean
	// shutdown). On a graceful shutdown both transports return ErrServerClosed-
	// style nil/wrapped errors that the .Serve methods translate to nil.
	if err := <-errCh; err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func splitCSV(s string) []string {
	out := []string{}
	cur := []byte{}
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if len(cur) > 0 {
				out = append(out, string(cur))
				cur = cur[:0]
			}
			continue
		}
		cur = append(cur, s[i])
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}
