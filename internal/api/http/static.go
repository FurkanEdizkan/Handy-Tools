package http

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/furkandedizkan/handy-tools/web"
)

// spaHandler serves the embedded Svelte SPA from web/dist.
//
//   - GET /assets/* (and any other path resolving to a real file in the embed)
//     is served verbatim with the right Content-Type via http.FileServer.
//   - Any other path falls back to index.html so client-side hash routing works
//     for deep links (refreshing on /tool/convert must return the SPA, not a
//     404).
//   - This handler is mounted on "GET /" — Go 1.22's ServeMux gives more
//     specific patterns priority, so the registered /v1/* routes win.
func newSPAHandler() (http.Handler, error) {
	dist, err := fs.Sub(web.FS(), "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "" || clean == "." {
			serveIndex(w, r, dist)
			return
		}

		if f, err := dist.Open(clean); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Reserve the /v1/ namespace for the API. A GET that reaches this
		// handler under /v1/ means the mux's specific GET patterns didn't
		// match (so the path is either POST-only or genuinely unknown).
		// Return 405 either way — it's the closest the SPA fallback can get
		// to the pre-SPA "method not allowed" behaviour without re-deriving
		// the registered route table here. The SPA must never paint over
		// API responses.
		if strings.HasPrefix(clean, "v1/") {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		serveIndex(w, r, dist)
	}), nil
}

func serveIndex(w http.ResponseWriter, _ *http.Request, dist fs.FS) {
	// Prefer the real Vite output; fall back to the committed placeholder
	// (kept outside dist/ so it survives `vite build` rebuilds).
	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		data = web.Placeholder()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Index is the SPA entry; never cache it so a redeploy is picked up on
	// next navigation. Hashed assets under /assets/ stay cacheable.
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}
