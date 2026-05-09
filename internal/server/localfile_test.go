package server

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAllowed_PathTraversalGuard exhaustively probes the path-traversal
// guard. Anything outside a registered root MUST be rejected, even if the
// attacker uses .. segments, double slashes, mixed case, or pre-resolved
// absolute paths.
func TestAllowed_PathTraversalGuard(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "videos")
	outside := filepath.Join(tmp, "secrets")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	insideFile := filepath.Join(root, "ok.mp4")
	if err := os.WriteFile(insideFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	siblingTrap := filepath.Join(tmp, "videos-evil", "leak.mp4")
	if err := os.MkdirAll(filepath.Dir(siblingTrap), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(siblingTrap, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewLocalFileHandler(root)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"file directly inside root", insideFile, true},
		{"root itself", root, true},
		{"file outside root", filepath.Join(outside, "x.mp4"), false},
		{"sibling whose name STARTS WITH root path (videos vs videos-evil)", siblingTrap, false},
		{"traversal via .. segments", filepath.Join(root, "..", "secrets", "x.mp4"), false},
		{"traversal via ../.. then back", filepath.Join(root, "..", "..", filepath.Base(tmp), "secrets", "x.mp4"), false},
		{"empty path", "", false},
		{"relative path that resolves outside cwd", "..", false},
	}

	if runtime.GOOS == "windows" {
		// Case-insensitive match is required for Windows where both UPPER
		// and lower drive paths refer to the same file.
		tests = append(tests, struct {
			name string
			path string
			want bool
		}{"windows case-insensitive prefix", strings.ToUpper(insideFile), true})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.allowed(tt.path)
			if got != tt.want {
				t.Errorf("allowed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestRegisterRoot_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	h := &LocalFileHandler{}
	h.RegisterRoot(tmp)
	h.RegisterRoot(tmp)
	h.RegisterRoot(strings.ToUpper(tmp)) // case-insensitive dedupe (Windows)
	if got := len(h.Roots()); got != 1 {
		t.Errorf("len(Roots) = %d after 3 registers of same dir, want 1", got)
	}
}

func TestRegisterRoot_IgnoresEmpty(t *testing.T) {
	h := &LocalFileHandler{}
	h.RegisterRoot("")
	if got := len(h.Roots()); got != 0 {
		t.Errorf("empty path should be ignored, but got %d roots", got)
	}
}

// TestServeHTTP_RejectsTraversal goes through the full handler pipeline so
// the URL decoding + allowed check + 403 status are all exercised.
func TestServeHTTP_RejectsTraversal(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "videos")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(tmp, "secrets", "passwd")
	if err := os.MkdirAll(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("toplevel"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewLocalFileHandler(root)

	enc := base64.RawURLEncoding.EncodeToString([]byte(outside))
	req := httptest.NewRequest("GET", "/localfile/"+enc, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 forbidden for path outside roots", rec.Code)
	}
}

func TestServeHTTP_ServesAllowedFile(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "videos")
	_ = os.MkdirAll(root, 0o755)
	target := filepath.Join(root, "v.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := NewLocalFileHandler(root)

	enc := base64.RawURLEncoding.EncodeToString([]byte(target))
	req := httptest.NewRequest("GET", "/localfile/"+enc, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "hello" {
		t.Errorf("body = %q, want %q", got, "hello")
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

func TestServeHTTP_RejectsBadEncoding(t *testing.T) {
	h := NewLocalFileHandler(t.TempDir())
	req := httptest.NewRequest("GET", "/localfile/not-base64!", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestServeHTTP_RejectsEmptyPath(t *testing.T) {
	h := NewLocalFileHandler(t.TempDir())
	req := httptest.NewRequest("GET", "/localfile/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
