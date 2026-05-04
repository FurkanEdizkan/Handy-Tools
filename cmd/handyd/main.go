// Command handyd is the gRPC server. It exposes the same tool packages the
// TUI uses (image, archive, pdf) over gRPC so Handy can run as a service.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/furkandedizkan/handy/internal/config"
	"github.com/furkandedizkan/handy/internal/server"
)

func main() {
	listen := flag.String("listen", "", "address to listen on (overrides config)")
	allow := flag.String("allow-roots", "", "comma-separated allow-list of filesystem roots; overrides config")
	flag.Parse()

	cfg, _, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	addr := cfg.Server.Listen
	if *listen != "" {
		addr = *listen
	}
	roots := cfg.Server.AllowRoots
	if *allow != "" {
		roots = splitCSV(*allow)
	}
	if len(roots) == 0 {
		fmt.Fprintln(os.Stderr,
			"refusing to start: no allow_roots configured. Set --allow-roots or server.allow_roots in config.yaml.")
		os.Exit(2)
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
	srv := server.New(server.Options{AllowRoots: roots})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		log.Println("shutting down")
		srv.Stop()
	}()

	log.Printf("handyd listening on %s; roots=%v", addr, roots)
	if err := srv.Serve(lis); err != nil {
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
