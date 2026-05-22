//go:build wails

// Command htools-gui wraps the Handy Tools Svelte frontend and Go core in a
// native desktop window via Wails. It serves the same embedded SPA and /v1/*
// HTTP API as htoolsd, but through an in-process Wails AssetServer — there is
// no network listener.
package main

import (
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	httpapi "github.com/furkandedizkan/handy-tools/internal/api/http"
	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/server"
)

func main() {
	// A desktop app runs on the user's own machine, so there is no path
	// sandbox: AllowRoots is "/" and the native file picker (#80) hands the
	// tools real absolute paths the user already has access to. The desktop
	// build needs no browser-upload staging, so the upload Manager is nil.
	api := httpapi.New(server.Options{AllowRoots: []string{"/"}}, queue.New(), nil)

	app := newApp()
	if err := wails.Run(&options.App{
		Title:  "Handy Tools",
		Width:  1100,
		Height: 720,
		AssetServer: &assetserver.Options{
			// internal/api/http already serves the embedded Svelte SPA and the
			// /v1/* API from one mux; the webview talks straight to it.
			Handler: api.Handler(),
		},
		Menu:      appMenu(app),
		OnStartup: app.startup,
		Bind:      []any{app},
	}); err != nil {
		log.Fatalf("htools-gui: %v", err)
	}
}
