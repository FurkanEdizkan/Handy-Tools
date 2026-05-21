package http

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
	"github.com/furkandedizkan/handy-tools/internal/tools/pdf"
)

// previewMaxEntries bounds the in-memory thumbnail LRU.
const previewMaxEntries = 64

// previewCache is a small LRU of rendered PNG thumbnails keyed by ETag. It
// saves a re-decode / re-render when a client's HTTP cache is cold.
type previewCache struct {
	mu  sync.Mutex
	max int
	ll  *list.List
	m   map[string]*list.Element
}

type previewEntry struct {
	etag string
	png  []byte
}

func newPreviewCache(max int) *previewCache {
	return &previewCache{max: max, ll: list.New(), m: make(map[string]*list.Element)}
}

func (c *previewCache) get(etag string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[etag]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*previewEntry).png, true
	}
	return nil, false
}

func (c *previewCache) put(etag string, png []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[etag]; ok {
		c.ll.MoveToFront(el)
		return
	}
	c.m[etag] = c.ll.PushFront(&previewEntry{etag: etag, png: png})
	for c.ll.Len() > c.max {
		back := c.ll.Back()
		if back == nil {
			break
		}
		c.ll.Remove(back)
		delete(c.m, back.Value.(*previewEntry).etag)
	}
}

// handlePreview renders a downscaled PNG thumbnail of an image or PDF.
// Caching: a strong ETag (path + mtime + size + bounds) drives client 304s,
// and a small server-side LRU avoids re-rendering on a cold client cache.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	raw := q.Get("path")
	if raw == "" {
		writeError(w, &tools.Error{Code: tools.CodeBadRequest, Message: "missing ?path"})
		return
	}
	path, err := s.Opts.CheckPath(raw)
	if err != nil {
		writeError(w, err)
		return
	}
	maxW := atoiOr(q.Get("w"), 0)
	maxH := atoiOr(q.Get("h"), 0)

	info, err := os.Stat(path) //nolint:gosec // path is sanitized by Opts.CheckPath
	if err != nil {
		writeError(w, &tools.Error{Code: tools.CodeIO, Message: "stat failed", Detail: err.Error()})
		return
	}
	etag := previewETag(path, info.ModTime().UnixNano(), info.Size(), maxW, maxH)

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if png, ok := s.previews.get(etag); ok {
		writePreview(w, etag, png)
		return
	}

	png, terr := renderPreview(r.Context(), path, maxW, maxH)
	if terr != nil {
		writeError(w, terr)
		return
	}
	s.previews.put(etag, png)
	writePreview(w, etag, png)
}

// renderPreview dispatches by file extension to the image or PDF previewer.
func renderPreview(ctx context.Context, path string, maxW, maxH int) ([]byte, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		res, err := pdf.Preview(ctx, path)
		if err != nil {
			return nil, err
		}
		return res.PNG, nil
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff", ".webp", ".heic", ".heif":
		res, err := image.Preview(image.PreviewRequest{Source: path, MaxWidth: maxW, MaxHeight: maxH})
		if err != nil {
			return nil, err
		}
		return res.PNG, nil
	}
	return nil, &tools.Error{Code: tools.CodeUnsupportedInput, Message: "preview: unsupported file type"}
}

func writePreview(w http.ResponseWriter, etag string, png []byte) {
	h := w.Header()
	h.Set("Content-Type", "image/png")
	h.Set("ETag", etag)
	h.Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

// previewETag is a strong validator over the inputs that change the output.
func previewETag(path string, mtime, size int64, w, h int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%d|%d", path, mtime, size, w, h)))
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n >= 0 {
		return n
	}
	return def
}
