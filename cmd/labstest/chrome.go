package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-rod/rod"
)

func findLabsTab(b *rod.Browser) (*rod.Page, string, error) {
	pages, err := b.Pages()
	if err != nil {
		return nil, "", err
	}
	for _, p := range pages {
		info, _ := p.Info()
		if info == nil || !strings.Contains(info.URL, "labs.google") {
			continue
		}
		u, _ := url.Parse(info.URL)
		parts := strings.Split(u.Path, "/")
		pid := ""
		for i := 0; i < len(parts)-1; i++ {
			if parts[i] == "project" {
				pid = parts[i+1]
				break
			}
		}
		return p, pid, nil
	}
	return nil, "", fmt.Errorf("no labs.google tab open")
}

func readToken(p *rod.Page) (string, error) {
	res, err := p.Eval(`() => {
		const el = document.getElementById('__NEXT_DATA__');
		if (!el) return '';
		const data = JSON.parse(el.textContent);
		return data?.props?.pageProps?.session?.access_token || '';
	}`)
	if err != nil {
		return "", err
	}
	var tok string
	if err := res.Value.Unmarshal(&tok); err != nil {
		return "", err
	}
	return tok, nil
}

func getRecaptchaToken(p *rod.Page) (string, error) {
	js := fmt.Sprintf(`
	async () => {
	  if (!window.grecaptcha || !window.grecaptcha.enterprise) {
	    throw new Error('grecaptcha.enterprise not loaded');
	  }
	  return await window.grecaptcha.enterprise.execute(%q, { action: %q });
	}
	`, recaptchaSiteKey, recaptchaAction)
	res, err := p.Eval(js)
	if err != nil {
		return "", err
	}
	var tok string
	if err := res.Value.Unmarshal(&tok); err != nil {
		return "", err
	}
	return tok, nil
}

// pageFetch executes fetch(url, opts) inside the page and returns the raw
// response body as string. Status >= 400 returns an error containing the body.
func pageFetch(p *rod.Page, method, fullURL, token, body string) ([]byte, error) {
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
	  if (%q.length) opts.body = %q;
	  const r = await fetch(%q, opts);
	  const text = await r.text();
	  return { status: r.status, body: text };
	}
	`, method, token, body, body, fullURL)

	res, err := p.Eval(js)
	if err != nil {
		return nil, fmt.Errorf("page eval fetch: %w", err)
	}
	var wrap struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	if err := res.Value.Unmarshal(&wrap); err != nil {
		return nil, fmt.Errorf("decode fetch wrapper: %w", err)
	}
	if wrap.Status >= 400 {
		return nil, fmt.Errorf("status %d: %s", wrap.Status, snip(wrap.Body))
	}
	return []byte(wrap.Body), nil
}

func snip(s string) string {
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
