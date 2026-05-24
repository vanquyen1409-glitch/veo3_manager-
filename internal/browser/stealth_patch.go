package browser

import (
	"context"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// stealthScript is injected into every page via Page.AddScriptToEvaluateOnNewDocument.
// It runs BEFORE any page script — so Google account login can't see automation
// markers when its frame mounts.
//
// Covers what go-rod/stealth misses (cdc_*) plus reinforces the basics in case
// rod opens a tab without going through stealth.Page().
const stealthScript = `
(() => {
  // 1. navigator.webdriver hard override
  Object.defineProperty(Navigator.prototype, 'webdriver', { get: () => undefined });

  // 2. window.chrome.runtime presence (signals real Chrome, not headless)
  if (!window.chrome) window.chrome = {};
  if (!window.chrome.runtime) window.chrome.runtime = { id: 'rod-stealth' };

  // 3. navigator.plugins length spoof (headless = 0)
  Object.defineProperty(Navigator.prototype, 'plugins', {
    get: () => {
      const arr = [
        { name: 'PDF Viewer',          filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
        { name: 'Chrome PDF Viewer',   filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
        { name: 'Chromium PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
      ];
      arr.refresh = () => {};
      arr.item = (i) => arr[i];
      arr.namedItem = (n) => arr.find((p) => p.name === n);
      return arr;
    },
  });

  // 4. languages
  Object.defineProperty(Navigator.prototype, 'languages', { get: () => ['en-US', 'en'] });

  // 5. permissions.query → match real Chrome (notifications return 'default')
  const origQuery = window.navigator.permissions && window.navigator.permissions.query;
  if (origQuery) {
    window.navigator.permissions.query = (params) => (
      params && params.name === 'notifications'
        ? Promise.resolve({ state: Notification.permission || 'default' })
        : origQuery.call(window.navigator.permissions, params)
    );
  }

  // 6. Strip ChromeDriver markers (cdc_*) — selenium fingerprints
  for (const k of Object.keys(window)) {
    if (/^cdc_/.test(k) || /^_phantom/.test(k) || /^__nightmare/.test(k)) {
      try { delete window[k]; } catch {}
    }
  }
  for (const k of Object.keys(document)) {
    if (/^cdc_/.test(k) || /^\$cdc_/.test(k)) {
      try { delete document[k]; } catch {}
    }
  }

  // 7. WebGL vendor + renderer spoof (headless returns 'Mesa', 'SwiftShader')
  try {
    const getParam = WebGLRenderingContext.prototype.getParameter;
    WebGLRenderingContext.prototype.getParameter = function (p) {
      if (p === 37445) return 'Intel Inc.';                  // UNMASKED_VENDOR_WEBGL
      if (p === 37446) return 'Intel Iris OpenGL Engine';    // UNMASKED_RENDERER_WEBGL
      return getParam.call(this, p);
    };
  } catch {}

  // 8. Iframe contentWindow unwrapping (some detectors wrap iframes)
  try {
    const desc = Object.getOwnPropertyDescriptor(HTMLIFrameElement.prototype, 'contentWindow');
    if (desc && desc.get) {
      Object.defineProperty(HTMLIFrameElement.prototype, 'contentWindow', {
        get() { return desc.get.call(this); },
      });
    }
  } catch {}

  // 9. Hardware concurrency (headless reports 1)
  Object.defineProperty(Navigator.prototype, 'hardwareConcurrency', { get: () => 8 });

  // 10. Device memory
  Object.defineProperty(Navigator.prototype, 'deviceMemory', { get: () => 8 });
})();
`

// applyBrowserStealth installs the stealth payload + UA override at the browser
// level so EVERY new page (including iframes used by Google login) inherits it.
//
// rod's stealth.Page() only patches a single page's "new document" script; this
// function does the same via Target.setAutoAttach + Page.addScriptToEvaluateOnNewDocument
// applied to existing pages, plus a custom UA.
//
// ctx controls the lifetime of the background TargetCreated listener; cancel
// it (Service.Stop) to terminate the goroutine cleanly. Without ctx-driven
// cancellation, repeated connect/disconnect cycles leak one goroutine each.
//
// fallbackUA is used only when the browser does not report a usable UA of its
// own (see below); pass realisticUA(chromePath) so it tracks the install.
func applyBrowserStealth(ctx context.Context, b *rod.Browser, fallbackUA string) error {
	// Prefer the browser's OWN reported UA — it is the genuine, current Chrome
	// string and is guaranteed to match the safe-login window. Only fall back
	// to the detected/static UA if it is missing or still leaks "Headless".
	ua := fallbackUA
	if v, err := b.Version(); err == nil && v.UserAgent != "" && !strings.Contains(v.UserAgent, "Headless") {
		ua = v.UserAgent
	}

	// Override UA at the network layer so every request — including the
	// initial Google account fetch — uses a stock Chrome UA.
	uaOverride := proto.NetworkSetUserAgentOverride{
		UserAgent:      ua,
		AcceptLanguage: "en-US,en;q=0.9",
		Platform:       "Win32",
	}

	// Apply to every existing page.
	pages, err := b.Pages()
	if err != nil {
		return err
	}
	for _, p := range pages {
		_, _ = p.EvalOnNewDocument(stealthScript)
		_ = uaOverride.Call(p)
	}

	// Hook future pages: when rod creates a new page, repeat the patches.
	// b.Context(ctx) makes EachEvent terminate when ctx is cancelled, so the
	// goroutine exits cleanly on Service.Stop instead of leaking on every
	// reconnect.
	go func() {
		b.Context(ctx).EachEvent(func(e *proto.TargetTargetCreated) {
			if e.TargetInfo == nil || e.TargetInfo.Type != proto.TargetTargetInfoTypePage {
				return
			}
			// Give the new page a moment to attach a session.
			pages2, err := b.Pages()
			if err != nil {
				return
			}
			for _, p := range pages2 {
				if string(p.TargetID) != string(e.TargetInfo.TargetID) {
					continue
				}
				_, _ = p.EvalOnNewDocument(stealthScript)
				_ = uaOverride.Call(p)
			}
		})()
	}()

	return nil
}
