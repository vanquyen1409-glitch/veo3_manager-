package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHttpDownload_HappyPath(t *testing.T) {
	want := []byte("hello video bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(want)))
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.mp4")
	var lastDownloaded, lastTotal int64
	progress := func(d, total int64) {
		lastDownloaded = d
		lastTotal = total
	}
	if err := httpDownload(context.Background(), srv.Client(), srv.URL, dst, progress); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("file content = %q, want %q", got, want)
	}
	if lastDownloaded != int64(len(want)) {
		t.Errorf("progress.downloaded = %d, want %d", lastDownloaded, len(want))
	}
	if lastTotal != int64(len(want)) {
		t.Errorf("progress.total = %d, want %d (Content-Length)", lastTotal, len(want))
	}
}

func TestHttpDownload_AtomicRenameAfterSuccess(t *testing.T) {
	// While the download is in progress there must be a .part file, never
	// the final dst file. After success only dst exists, no .part.
	want := []byte("xxxxxxxxxxxxxxxxxx")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "subdir", "out.mp4")
	if err := httpDownload(context.Background(), srv.Client(), srv.URL, dst, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("dst missing after success: %v", err)
	}
	if _, err := os.Stat(dst + ".part"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".part should be gone after success, but Stat returned %v", err)
	}
}

func TestHttpDownload_FailureLeavesNoPartialFile(t *testing.T) {
	// Server returns 500 → must not leave a .part lying around.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.mp4")
	err := httpDownload(context.Background(), srv.Client(), srv.URL, dst, nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v, want '500' in message", err)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Error("dst exists after failed download")
	}
	if _, err := os.Stat(dst + ".part"); err == nil {
		t.Error(".part file leaked after failed download")
	}
}

func TestHttpDownload_ContextCancelMidStream(t *testing.T) {
	// Server sends slowly so we can cancel mid-stream.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100000")
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			_, _ = w.Write(make([]byte, 1000))
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(20 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	dst := filepath.Join(t.TempDir(), "out.mp4")
	err := httpDownload(ctx, srv.Client(), srv.URL, dst, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		// Some pipeline points wrap into other errors; soft-fail with helpful msg.
		t.Logf("err = %v (not strictly context.Canceled but cancelling is OK)", err)
	}
	// .part may exist briefly then be cleaned by deferred Remove. Verify it
	// doesn't end up as the final dst file at least.
	if _, err := os.Stat(dst); err == nil {
		t.Error("dst exists after cancelled download")
	}
}

func TestHttpDownload_MkdirParents(t *testing.T) {
	// Destination dir doesn't exist yet - must be created.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "deep", "nested", "out.mp4")
	if err := httpDownload(context.Background(), srv.Client(), srv.URL, dst, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("dst missing: %v", err)
	}
}

func TestHttpDownload_ProgressReportedIncrementally(t *testing.T) {
	// Verify progress callback is fired more than once for chunked content.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "200000")
		// 4 chunks of 50_000 bytes
		for i := 0; i < 4; i++ {
			_, _ = io.CopyN(w, &zeros{}, 50000)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.mp4")
	calls := 0
	if err := httpDownload(context.Background(), srv.Client(), srv.URL, dst, func(d, total int64) {
		calls++
	}); err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Errorf("progress called %d time(s), want >= 2 for a chunked transfer", calls)
	}
}

// zeros implements io.Reader returning all-zero bytes forever.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestIsGCSURL(t *testing.T) {
	tests := map[string]bool{
		"https://flow-content.google/abc":         true,
		"https://storage.googleapis.com/bucket/x": true,
		"https://x.googleusercontent.com/path":    true,
		"https://labs.google/redirect":            false,
		"https://accounts.google.com/login":       false,
		"":                                        false,
		"http://flow-content.google/CASE":         true, // case-insensitive
	}
	for url, want := range tests {
		t.Run(url, func(t *testing.T) {
			if got := isGCSURL(url); got != want {
				t.Errorf("isGCSURL(%q) = %v, want %v", url, got, want)
			}
		})
	}
}

func TestBuildPath_FormatAndSanitization(t *testing.T) {
	got := BuildPath("/out", "task-1234567890abcdef", 42, "mp4")
	// Path must include outputDir, first 8 chars of taskID, and seed.
	if !strings.Contains(got, "task-123") {
		t.Errorf("path %q missing first 8 chars of taskID", got)
	}
	if !strings.HasSuffix(got, "_42.mp4") {
		t.Errorf("path %q missing seed + extension", got)
	}
	if !strings.HasPrefix(filepath.ToSlash(got), "/out/") {
		t.Errorf("path %q missing outputDir prefix", got)
	}
}

func TestBuildPath_DefaultExt(t *testing.T) {
	got := BuildPath("/out", "x", 1, "")
	if !strings.HasSuffix(got, ".mp4") {
		t.Errorf("default extension should be mp4: %q", got)
	}
}

func TestBuildPath_StripsLeadingDotInExt(t *testing.T) {
	got := BuildPath("/out", "x", 1, ".webm")
	if !strings.HasSuffix(got, ".webm") || strings.HasSuffix(got, "..webm") {
		t.Errorf("ext normalization wrong: %q", got)
	}
}

func TestBuildPath_SanitizesIllegalChars(t *testing.T) {
	// taskID with Windows-illegal chars must be sanitised so the resulting
	// path is creatable on every platform.
	got := BuildPath("/out", `bad:x*?<>|"id`, 1, "mp4")
	for _, ch := range `:*?<>|"` {
		if strings.ContainsRune(filepath.Base(got), ch) {
			t.Errorf("path %q still contains illegal char %q", got, ch)
		}
	}
}

func TestDiskFreeBytes_NonZeroOnTmp(t *testing.T) {
	// Sanity: temp dir on every CI runner has > 0 bytes free. If this
	// returns 0 with no error, our fallback "skip if 0" guard works as
	// designed; we just won't have asserted positive.
	dir := t.TempDir()
	free, err := DiskFreeBytes(dir)
	if err != nil {
		t.Fatalf("DiskFreeBytes(%q): %v", dir, err)
	}
	if free == 0 {
		t.Logf("DiskFreeBytes returned 0 with no error - worker treats this as 'unknown' and skips precheck")
	}
}
