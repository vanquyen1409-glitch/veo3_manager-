package download

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// BrowserSource is the minimal browser surface the downloader needs.
type BrowserSource interface {
	NewTab(ctx context.Context, url string) (*rod.Page, error)
}

// Downloader resolves redirect URLs to signed GCS URLs (using a logged-in
// Chrome tab) and downloads them via plain HTTP.
type Downloader struct {
	browser BrowserSource
	http    *http.Client
}

func New(b BrowserSource) *Downloader {
	return &Downloader{browser: b, http: defaultClient()}
}

// Fetch resolves redirectURL and downloads the resulting file to dstPath.
func (d *Downloader) Fetch(ctx context.Context, redirectURL, dstPath string, onProgress ProgressFn) error {
	gcs, err := d.Resolve(ctx, redirectURL)
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	return httpDownload(ctx, d.http, gcs, dstPath, onProgress)
}

// Download is the second-half-only path when caller already has the GCS URL.
func (d *Downloader) Download(ctx context.Context, gcsURL, dstPath string, onProgress ProgressFn) error {
	return httpDownload(ctx, d.http, gcsURL, dstPath, onProgress)
}

// Resolve opens the redirect URL in a hidden Chrome tab and captures the
// final signed GCS URL via NetworkResponseReceived. Falls back to reading
// page.Info().URL when the listener path doesn't fire.
func (d *Downloader) Resolve(ctx context.Context, redirectURL string) (string, error) {
	page, err := d.browser.NewTab(ctx, "about:blank")
	if err != nil {
		return "", fmt.Errorf("new tab: %w", err)
	}
	defer func() { _ = page.Close() }()

	// Enable network domain so we receive Response events.
	enable := proto.NetworkEnable{}
	if err := enable.Call(page); err != nil {
		return "", fmt.Errorf("network enable: %w", err)
	}

	gcsCh := make(chan string, 1)
	stop := page.EachEvent(func(e *proto.NetworkResponseReceived) {
		u := e.Response.URL
		if isGCSURL(u) {
			select {
			case gcsCh <- u:
			default:
			}
		}
	})
	go stop()

	if err := page.Navigate(redirectURL); err != nil {
		return "", fmt.Errorf("navigate: %w", err)
	}

	// flow-content.google returns a tiny HTML wrapper that auto-plays the
	// signed video URL; the page's final location IS the signed URL we want.
	// Wait for navigation to settle, then check.
	_ = page.WaitLoad()

	timeout := 20 * time.Second
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case u := <-gcsCh:
		return u, nil
	case <-t.C:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Fallback 1: page final URL.
	if info, err := page.Info(); err == nil && info != nil && isGCSURL(info.URL) {
		return info.URL, nil
	}
	// Fallback 2: extract <video src> or <a href> matching a signed CDN URL.
	res, evalErr := page.Eval(`() => {
		const v = document.querySelector('video source, video');
		if (v) return v.src || v.currentSrc || '';
		const a = document.querySelector('a[href*="flow-content.google"], a[href*="googleusercontent.com"], a[href*="storage.googleapis.com"]');
		if (a) return a.href || '';
		return '';
	}`)
	if evalErr == nil && res != nil {
		var u string
		if err := res.Value.Unmarshal(&u); err == nil && isGCSURL(u) {
			return u, nil
		}
	}
	return "", errors.New("signed video URL not captured")
}

func isGCSURL(u string) bool {
	if u == "" {
		return false
	}
	low := strings.ToLower(u)
	return strings.Contains(low, "flow-content.google/") ||
		strings.Contains(low, "storage.googleapis.com/") ||
		strings.Contains(low, "googleusercontent.com/")
}
