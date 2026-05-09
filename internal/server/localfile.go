package server

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LocalFileHandler serves files under /localfile/<base64url>. The decoded path
// must live inside one of the registered roots (path-traversal protection).
type LocalFileHandler struct {
	mu    sync.RWMutex
	roots []string
}

// NewLocalFileHandler builds a handler with the given initial roots.
func NewLocalFileHandler(initialRoots ...string) *LocalFileHandler {
	h := &LocalFileHandler{}
	for _, r := range initialRoots {
		h.RegisterRoot(r)
	}
	return h
}

// RegisterRoot adds a directory to the allow-list. Idempotent.
func (h *LocalFileHandler) RegisterRoot(p string) {
	if p == "" {
		return
	}
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.roots {
		if strings.EqualFold(r, abs) {
			return
		}
	}
	h.roots = append(h.roots, abs)
}

// Roots returns a copy of the current allow-list (for diagnostics).
func (h *LocalFileHandler) Roots() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, len(h.roots))
	copy(out, h.roots)
	return out
}

func (h *LocalFileHandler) allowed(p string) bool {
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return false
	}
	lower := strings.ToLower(abs)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, r := range h.roots {
		rl := strings.ToLower(r)
		if lower == rl {
			return true
		}
		if strings.HasPrefix(lower, rl+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ServeHTTP serves a single file decoded from the URL path.
func (h *LocalFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	enc := strings.TrimPrefix(r.URL.Path, "/localfile/")
	enc = strings.TrimSpace(enc)
	if enc == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		http.Error(w, "bad path encoding", http.StatusBadRequest)
		return
	}
	p := string(raw)
	if !h.allowed(p) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, p)
}
