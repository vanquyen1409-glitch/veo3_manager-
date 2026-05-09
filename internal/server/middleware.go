package server

import (
	"net/http"
	"strings"
)

// LocalFilePrefix is the URL prefix the frontend uses to address local files.
const LocalFilePrefix = "/localfile/"

// Middleware wraps a Wails AssetServer chain so any /localfile/ request is
// served by h, while everything else passes through to next.
func Middleware(h http.Handler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, LocalFilePrefix) {
				h.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
