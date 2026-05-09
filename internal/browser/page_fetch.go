package browser

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// reCAPTCHA Enterprise constants captured from labs.google.
const (
	RecaptchaSiteKey = "6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV"
	RecaptchaAction  = "VIDEO_GENERATION"
)

// ProjectID extracts the Flow project UUID from the labs.google tab URL,
// e.g. https://labs.google/fx/vi/tools/flow/project/<uuid>/edit/<wf>.
// Returns "" when no project segment is in the URL (caller must navigate
// to a project page first).
func (s *Service) ProjectID() string {
	s.mu.RLock()
	page := s.page
	s.mu.RUnlock()
	if page == nil {
		return ""
	}
	info, err := page.Info()
	if err != nil || info == nil {
		return ""
	}
	u, err := url.Parse(info.URL)
	if err != nil {
		return ""
	}
	parts := strings.Split(u.Path, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "project" {
			return parts[i+1]
		}
	}
	return ""
}

// RecaptchaToken executes grecaptcha.enterprise on the labs.google page and
// returns a fresh token. Tokens expire in ~120 s — fetch one per submit.
func (s *Service) RecaptchaToken(ctx context.Context) (string, error) {
	s.mu.RLock()
	page := s.page
	s.mu.RUnlock()
	if page == nil {
		return "", ErrNotConnected
	}
	js := fmt.Sprintf(`
	async () => {
	  if (!window.grecaptcha || !window.grecaptcha.enterprise) {
	    throw new Error('grecaptcha.enterprise not loaded');
	  }
	  return await window.grecaptcha.enterprise.execute(%q, { action: %q });
	}
	`, RecaptchaSiteKey, RecaptchaAction)
	res, err := page.Eval(js)
	if err != nil {
		return "", fmt.Errorf("recaptcha eval: %w", err)
	}
	var tok string
	if err := res.Value.Unmarshal(&tok); err != nil {
		return "", err
	}
	if tok == "" {
		return "", fmt.Errorf("empty recaptcha token")
	}
	return tok, nil
}

// PageFetch issues an HTTP request from inside the labs.google tab via
// `fetch()`. Required because aisandbox-pa's reCAPTCHA Enterprise check
// validates the request origin/IP — direct Go http.Client calls fail with
// "PUBLIC_ERROR_UNUSUAL_ACTIVITY" even with a fresh token.
//
// method   : "GET" / "POST"
// fullURL  : e.g. https://aisandbox-pa.googleapis.com/v1/video:batchAsync...
// token    : Bearer access token (we re-pass it explicitly so the script
//
//	doesn't need access to the closure)
//
// jsonBody : marshalled request body (empty string for GET)
//
// Returns the raw response body bytes (the page's fetch().text()), or an
// error containing the body for non-2xx responses.
func (s *Service) PageFetch(ctx context.Context, method, fullURL, token, jsonBody string) ([]byte, error) {
	s.mu.RLock()
	page := s.page
	s.mu.RUnlock()
	if page == nil {
		return nil, ErrNotConnected
	}
	js := fmt.Sprintf(`
	async () => {
	  const opts = {
	    method: %q,
	    headers: {
	      'Authorization': 'Bearer ' + %q,
	      'Content-Type': 'text/plain;charset=UTF-8',
	      'Accept': 'application/json',
	    },
	    credentials: 'include',
	  };
	  if (%q.length > 0) opts.body = %q;
	  const r = await fetch(%q, opts);
	  const text = await r.text();
	  return { status: r.status, body: text };
	}
	`, method, token, jsonBody, jsonBody, fullURL)

	res, err := page.Eval(js)
	if err != nil {
		return nil, fmt.Errorf("page fetch: %w", err)
	}
	var wrap struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	if err := res.Value.Unmarshal(&wrap); err != nil {
		return nil, fmt.Errorf("decode wrapper: %w", err)
	}
	if wrap.Status >= 400 {
		body := wrap.Body
		if len(body) > 600 {
			body = body[:600] + "…"
		}
		// 401 is the recoverable case (token expired). Return a wrapped
		// sentinel so callers (labsapi.Client) can detect it with errors.Is
		// and trigger a token refresh + single retry.
		if wrap.Status == 401 {
			return nil, fmt.Errorf("%w: %s %s: %s", ErrUnauthorized, method, fullURL, body)
		}
		return nil, fmt.Errorf("labsapi %s %s: status %d: %s", method, fullURL, wrap.Status, body)
	}
	return []byte(wrap.Body), nil
}

