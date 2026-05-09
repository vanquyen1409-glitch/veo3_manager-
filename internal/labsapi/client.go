// Package labsapi drives the unofficial labs.google video generation API.
//
// All calls happen INSIDE the user's signed-in Chrome tab via PageFetch
// (page.evaluate(fetch)). Direct Go http.Client calls trip reCAPTCHA
// Enterprise's "PUBLIC_ERROR_UNUSUAL_ACTIVITY" because Google validates
// origin / IP / fingerprint of the request itself, not just the bearer.
package labsapi

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"veo3-manager/internal/browser"
)

// PageFetcher abstracts the labs.google tab. Implemented by *browser.Service.
type PageFetcher interface {
	// PageFetch issues fetch(method, fullURL, body) inside the labs.google
	// tab. token is the OAuth bearer extracted from __NEXT_DATA__.
	PageFetch(ctx context.Context, method, fullURL, token, jsonBody string) ([]byte, error)
	// Token returns a fresh bearer token (re-extracts on demand).
	Token(ctx context.Context) (string, error)
	// Refresh forces a token re-extraction (called on 401).
	Refresh(ctx context.Context) error
	// RecaptchaToken yields a single-use reCAPTCHA Enterprise token.
	RecaptchaToken(ctx context.Context) (string, error)
	// ProjectID returns the active Flow project UUID, or "" if none.
	ProjectID() string
	// EnsureProjectView normalises the tab URL so a Flow project root is
	// open. The Submit path calls this before reading ProjectID so a tab
	// that drifted to /tools/flow root or /edit/<wf> is auto-recovered.
	EnsureProjectView() error
}

// Client is the typed API surface used by the queue worker.
type Client struct {
	page PageFetcher
	log  *slog.Logger
}

// NewClient wires the API client to a labs.google page fetcher.
func NewClient(page PageFetcher, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{page: page, log: logger}
}

// PollOptions configures Wait. Zero values use sane defaults (10s / 5min).
type PollOptions struct {
	Interval time.Duration
	Timeout  time.Duration
}

// RandSeed returns a positive int32-range seed. The Labs API rejects int64
// values with "Invalid value at 'requests[i].seed' (TYPE_INT32)".
func RandSeed() int64 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	v := int64(binary.LittleEndian.Uint32(b[:]) & 0x7fffffff)
	if v == 0 {
		v = 1
	}
	return v
}

// fetchJSON marshals body, calls PageFetch, and unmarshals into out.
//
// Transparently retries once on 401 after refreshing the bearer token. This
// covers the common case where the cached token has just crossed Google's
// ~30-min expiry while the worker is between submit and poll. Without this,
// every long-running task would fail mid-flight and the user would have to
// requeue manually.
func (c *Client) fetchJSON(ctx context.Context, method, path string, body, out any) error {
	bodyJSON := ""
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyJSON = string(b)
	}

	doOnce := func() ([]byte, error) {
		token, err := c.page.Token(ctx)
		if err != nil {
			return nil, fmt.Errorf("token: %w", err)
		}
		started := time.Now()
		raw, err := c.page.PageFetch(ctx, method, BaseURL+path, token, bodyJSON)
		c.log.Debug("labsapi", "method", method, "path", path,
			"ms", time.Since(started).Milliseconds(), "err", err)
		return raw, err
	}

	raw, err := doOnce()
	if errors.Is(err, browser.ErrUnauthorized) {
		c.log.Info("labsapi 401 - refreshing token and retrying once",
			"method", method, "path", path)
		if rerr := c.page.Refresh(ctx); rerr != nil {
			return fmt.Errorf("token refresh after 401: %w (orig: %v)", rerr, err)
		}
		raw, err = doOnce()
	}
	if err != nil {
		return err
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode %s: %w (body: %s)", path, err, snip(raw))
		}
	}
	return nil
}

func snip(b []byte) string {
	const max = 400
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}

// GetCredits returns the user's paygate tier + remaining credit count.
func (c *Client) GetCredits(ctx context.Context) (tier string, credits int, err error) {
	var out struct {
		Credits         int    `json:"credits"`
		UserPaygateTier string `json:"userPaygateTier"`
	}
	if err := c.fetchJSON(ctx, "GET", PathCredits, nil, &out); err != nil {
		return "", 0, err
	}
	if out.UserPaygateTier == "" {
		out.UserPaygateTier = "PAYGATE_TIER_ONE"
	}
	return out.UserPaygateTier, out.Credits, nil
}
