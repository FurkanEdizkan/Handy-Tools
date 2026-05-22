// Package upload manages per-request temp workspaces for the browser-upload
// file converter. A plain browser cannot supply a server-side filesystem path
// the way the Wails desktop build can, so the HTTP transport stages uploaded
// bytes here: each upload gets an isolated workspace with an in/ directory
// (the uploaded files) and an out/ directory (where a tool writes its result).
//
// The workspace base directory is registered as an allow-root, so the existing
// tool endpoints run against staged files unchanged. Workspaces are reaped on a
// TTL so abandoned uploads do not leak disk.
package upload

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dirPerm is used for the base directory and every workspace tree. 0o700 keeps
// uploaded bytes readable only by the user running htoolsd.
const dirPerm = 0o700

// StoredFile is one uploaded file staged inside a workspace's in/ directory.
type StoredFile struct {
	Name string // sanitized base name as stored on disk
	Path string // absolute path inside the workspace in/ directory
}

// Workspace is the isolated temp tree for a single upload request.
type Workspace struct {
	ID        string // crypto-random, URL-safe identifier
	Root      string // {Manager.Base}/{ID}
	InDir     string // {Root}/in  — uploaded files land here
	OutDir    string // {Root}/out — a tool writes its result here
	CreatedAt time.Time
	Files     []StoredFile
}

// Manager owns the upload base directory and the set of live workspaces. All
// workspace directories live directly under Base, so Base is the single path a
// caller must register as an allow-root.
type Manager struct {
	Base     string        // absolute, cleaned base directory
	MaxBytes int64         // per-request total body cap, enforced by the HTTP layer
	TTL      time.Duration // idle lifetime before the reaper deletes a workspace

	mu     sync.Mutex
	spaces map[string]*Workspace
	now    func() time.Time // injectable clock for tests
}

// NewManager creates the base directory and returns a ready Manager. base may
// be relative; it is resolved to an absolute, cleaned path.
func NewManager(base string, maxBytes int64, ttl time.Duration) (*Manager, error) {
	if base == "" {
		return nil, errors.New("upload: empty base directory")
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("upload: resolve base: %w", err)
	}
	abs = filepath.Clean(abs)
	if err := os.MkdirAll(abs, dirPerm); err != nil {
		return nil, fmt.Errorf("upload: create base: %w", err)
	}
	return &Manager{
		Base:     abs,
		MaxBytes: maxBytes,
		TTL:      ttl,
		spaces:   make(map[string]*Workspace),
		now:      time.Now,
	}, nil
}

// Create allocates a fresh workspace with empty in/ and out/ directories.
func (m *Manager) Create() (*Workspace, error) {
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(m.Base, id)
	ws := &Workspace{
		ID:        id,
		Root:      root,
		InDir:     filepath.Join(root, "in"),
		OutDir:    filepath.Join(root, "out"),
		CreatedAt: m.now(),
	}
	for _, d := range []string{ws.InDir, ws.OutDir} {
		if err := os.MkdirAll(d, dirPerm); err != nil {
			_ = os.RemoveAll(root)
			return nil, fmt.Errorf("upload: create workspace: %w", err)
		}
	}
	m.mu.Lock()
	m.spaces[id] = ws
	m.mu.Unlock()
	return ws, nil
}

// Get returns a live workspace by ID.
func (m *Manager) Get(id string) (*Workspace, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws, ok := m.spaces[id]
	return ws, ok
}

// Save copies r into the workspace in/ directory under a sanitized form of
// name and records it on the workspace. The resolved path is asserted to stay
// inside InDir as defense-in-depth on top of SanitizeFilename.
func (m *Manager) Save(ws *Workspace, name string, r io.Reader) (StoredFile, error) {
	clean := SanitizeFilename(name)
	dst := filepath.Join(ws.InDir, clean)
	if !strings.HasPrefix(filepath.Clean(dst)+string(filepath.Separator),
		filepath.Clean(ws.InDir)+string(filepath.Separator)) {
		return StoredFile{}, fmt.Errorf("upload: refusing path outside workspace: %q", name)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return StoredFile{}, fmt.Errorf("upload: create staged file: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return StoredFile{}, fmt.Errorf("upload: write staged file: %w", err)
	}
	if err := f.Close(); err != nil {
		return StoredFile{}, fmt.Errorf("upload: close staged file: %w", err)
	}
	sf := StoredFile{Name: clean, Path: dst}
	m.mu.Lock()
	ws.Files = append(ws.Files, sf)
	m.mu.Unlock()
	return sf, nil
}

// Remove deletes a workspace and its files. It is idempotent — removing an
// unknown ID is not an error.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	ws, ok := m.spaces[id]
	delete(m.spaces, id)
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return os.RemoveAll(ws.Root)
}

// Reap deletes every workspace older than the TTL. It is safe to call
// concurrently with the rest of the Manager API.
func (m *Manager) Reap() {
	cutoff := m.now().Add(-m.TTL)
	m.mu.Lock()
	var stale []*Workspace
	for id, ws := range m.spaces {
		if ws.CreatedAt.Before(cutoff) {
			stale = append(stale, ws)
			delete(m.spaces, id)
		}
	}
	m.mu.Unlock()
	for _, ws := range stale {
		_ = os.RemoveAll(ws.Root)
	}
}

// StartReaper runs Reap on a ticker until ctx is cancelled. The interval is
// half the TTL so a workspace lives at most ~1.5×TTL in the worst case.
func (m *Manager) StartReaper(ctx context.Context) {
	interval := m.TTL / 2
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.Reap()
			}
		}
	}()
}

// SanitizeFilename reduces an arbitrary client-supplied name to a safe base
// name with no directory separators or traversal segments. It is the primary
// defense against path traversal via multipart filenames; an empty or unsafe
// result falls back to "upload".
func SanitizeFilename(name string) string {
	// Reject Windows-style separators too — filepath.Base on Linux would not.
	name = strings.ReplaceAll(name, "\\", "/")
	base := filepath.Base(name)
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == ".." || strings.ContainsRune(base, os.PathSeparator) {
		return "upload"
	}
	return base
}

// randomID returns a 16-byte crypto-random, lowercase base32 identifier with
// no padding — URL-safe and free of filesystem-special characters.
func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("upload: random id: %w", err)
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return strings.ToLower(enc.EncodeToString(b[:])), nil
}
