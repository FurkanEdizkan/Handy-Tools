// Command htools-mcp exposes the Handy Tools toolbox over the Model Context
// Protocol. It runs as a subprocess of an MCP-capable client (Claude Code,
// Claude Desktop, Cursor, ...) and speaks JSON-RPC over stdio.
//
// The binary reuses internal/server/*Handler types — the same wire-agnostic
// adapters that the gRPC and HTTP transports wrap. Path safety is enforced
// the same way (Options.CheckPath), but since the process runs as a
// subprocess of the local user, AllowRoots defaults to "/" — the same
// posture htools-gui uses.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/furkandedizkan/handy-tools/internal/buildinfo"
	"github.com/furkandedizkan/handy-tools/internal/server"
)

func main() {
	allow := flag.String("allow-roots", "/", "comma-separated allow-list of filesystem roots; default '/' (subprocess runs as local user)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(buildinfo.String())
		return
	}

	opts := server.Options{AllowRoots: splitCSV(*allow)}
	handlers := newHandlers(opts)

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "handy-tools",
		Version: buildinfo.Version,
	}, nil)

	registerPDFTools(srv, handlers)
	registerImageTools(srv, handlers)
	registerArchiveTools(srv, handlers)
	registerMiscTools(srv, handlers)

	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("htools-mcp: %v", err)
	}
}

// handlers bundles every server-layer adapter the MCP tools dispatch into.
// One value is built at startup so each registerXTools call can wire its
// closures against the same set of Opts-validated handlers.
type handlers struct {
	Image    *server.ImageHandler
	Archive  *server.ArchiveHandler
	PDF      *server.PDFHandler
	Hash     *server.HashHandler
	DiffTree *server.DiffTreeHandler
	Rename   *server.RenameHandler
	Opts     server.Options
}

func newHandlers(opts server.Options) *handlers {
	return &handlers{
		Image:    &server.ImageHandler{Opts: opts},
		Archive:  &server.ArchiveHandler{Opts: opts},
		PDF:      &server.PDFHandler{Opts: opts},
		Hash:     &server.HashHandler{Opts: opts},
		DiffTree: &server.DiffTreeHandler{Opts: opts},
		Rename:   &server.RenameHandler{Opts: opts},
		Opts:     opts,
	}
}

func splitCSV(s string) []string {
	out := make([]string, 0, 4)
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
