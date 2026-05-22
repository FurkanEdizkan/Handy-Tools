package http

import (
	"net/http"
	"strings"
)

// defaultCORSOrigins is the built-in allow-list used when no cors_origins are
// configured. It covers local development and the Chrome extension:
//   - http://localhost and http://127.0.0.1 on ANY port (the htoolsd HTTP
//     port, a Vite dev server, etc.);
//   - any chrome-extension:// origin — matched by scheme, because the
//     unpacked-extension id is unstable and the Web Store id is unknown until
//     publish. The extension already needs a user-granted host permission to
//     reach htoolsd, so scheme-level matching is an acceptable trust boundary.
var defaultCORSOrigins = []string{
	"http://localhost",
	"http://127.0.0.1",
	"chrome-extension:",
}

// corsMiddleware wraps next with cross-origin support for browser clients. A
// request whose Origin matches allowed (or the built-in default when allowed
// is empty) gets its Origin reflected back with the relevant Access-Control-*
// headers; a non-matching Origin gets none, so the browser fails the request
// closed. OPTIONS preflights are answered here with 204 so they never reach
// the route table.
func corsMiddleware(next http.Handler, allowed []string) http.Handler {
	if len(allowed) == 0 {
		allowed = defaultCORSOrigins
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			// Vary so a shared cache never serves one origin's CORS
			// response to another.
			w.Header().Add("Vary", "Origin")
		}
		if origin != "" && originAllowed(origin, allowed) {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Accept")
			h.Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			// Preflight: answer here regardless of route. Without a matching
			// Origin no Access-Control-* headers were set, so the browser
			// still rejects the real request — fail closed.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originAllowed reports whether a request Origin is permitted. An allow-list
// entry ending in ":" is a scheme sentinel (e.g. "chrome-extension:") matched
// by scheme prefix; otherwise it is a scheme+host entry (e.g.
// "http://localhost") matched exactly or with any port appended.
func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		switch {
		case a == "":
			continue
		case strings.HasSuffix(a, ":"):
			if strings.HasPrefix(origin, a) {
				return true
			}
		case origin == a || strings.HasPrefix(origin, a+":"):
			return true
		}
	}
	return false
}
