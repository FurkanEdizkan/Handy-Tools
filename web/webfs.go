// Package web exposes the built Svelte frontend as an embed.FS so the
// htoolsd binary can serve the SPA without external assets.
//
// The dist/ directory is produced by `make web` (or `cd web && npm run build`).
// A placeholder.html sibling is committed so `htoolsd` still serves something
// useful before the first build — contributors who only touch Go code don't
// need Node installed for `go build ./...` to work.
package web

import "embed"

//go:embed all:dist
var distFS embed.FS

//go:embed placeholder.html
var placeholderHTML []byte

// FS returns the embedded dist/ tree as a filesystem rooted at "dist".
// Callers typically wrap it with fs.Sub so paths in served URLs match files
// in the build output (i.e. "/index.html" → "dist/index.html").
func FS() embed.FS { return distFS }

// Placeholder is the static HTML shown when dist/index.html hasn't been built
// yet — kept out of dist/ so `vite build` (which empties dist) doesn't clobber
// it on every rebuild.
func Placeholder() []byte { return placeholderHTML }
