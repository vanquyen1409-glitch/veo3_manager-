package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
)

// BypassFlags lists Chrome launch flags this service applies to evade simple
// bot detection. Surfaced in the UI so users can verify what's active.
var BypassFlags = []string{
	"DELETED: --enable-automation (rod default — Google checks this)",
	"DELETED: --use-mock-keychain (rod default)",
	"--disable-blink-features=AutomationControlled",
	"--disable-features=AutomationControlled,IsolateOrigins,site-per-process,Translate,OptimizationGuideModelDownloading",
	"--enable-features=NetworkService,NetworkServiceInProcess",
	"--user-agent=Mozilla/5.0 ... Chrome/130.0.0.0 (real UA, no HeadlessChrome)",
	"--user-data-dir=<persistent profile, cookies survive restarts>",
	"--lang=en-US,en;q=0.9",
	"--password-store=basic",
	"--disable-component-extensions-with-background-pages",
	"--no-first-run",
	"--no-default-browser-check",
	"--disable-default-apps",
	"--remote-allow-origins=*",
}

// StealthPatches mirrors what we inject via Page.EvalOnNewDocument BEFORE any
// page script runs. Combines go-rod/stealth defaults plus our extra cdc_*
// kills targeted at Google account login.
var StealthPatches = []string{
	"navigator.webdriver → undefined (hard override on prototype)",
	"window.chrome.runtime → present (real-Chrome marker)",
	"navigator.plugins → spoofed (PDF + 2 Chrome PDF entries)",
	"navigator.languages → ['en-US','en']",
	"navigator.hardwareConcurrency → 8",
	"navigator.deviceMemory → 8",
	"navigator.permissions.query → spoofed for 'notifications'",
	"window['cdc_*'], document['cdc_*'] → deleted (ChromeDriver markers)",
	"window._phantom, window.__nightmare → deleted",
	"WebGL UNMASKED_VENDOR/RENDERER → 'Intel Inc.' / 'Intel Iris OpenGL Engine'",
	"HTMLIFrameElement.contentWindow → unwrapped (login uses iframes)",
	"Network.setUserAgentOverride → applied via CDP on every page",
	"go-rod/stealth bundle (chrome.runtime, plugins, etc.)",
}

// Detail builds the heavy info payload (profile size, UA, Chrome version, email).
// Safe to call without a connection — fields just remain empty.
func (s *Service) Detail() Detail {
	snap := s.Snapshot()

	d := Detail{Snapshot: snap}
	d.JsonVersionURL = ""
	if snap.DebugPort > 0 {
		d.JsonVersionURL = fmt.Sprintf("http://127.0.0.1:%d/json/version", snap.DebugPort)
	}
	if !s.connectedAt.IsZero() {
		d.ConnectedAtMs = s.connectedAt.UnixMilli()
	}

	d.StealthEnabled = true
	d.BypassFlags = BypassFlags
	d.StealthPatches = StealthPatches

	// Profile dir size — best effort, capped traversal time.
	d.ProfileSizeBytes = profileSize(snap.UserDataDir)

	s.mu.RLock()
	br := s.browser
	page := s.page
	s.mu.RUnlock()

	if br != nil {
		if pages, err := br.Pages(); err == nil {
			d.ProfileTabs = len(pages)
		}
		if v, err := br.Version(); err == nil && v != nil {
			d.ChromeVersion = v.Product
			d.UserAgent = v.UserAgent
		}
	}

	if page != nil {
		d.LoginEmail = probeEmail(page)
	}

	return d
}

// probeEmail tries to read the signed-in Google account email from the page.
// Returns "" silently if not detectable.
func probeEmail(p *rod.Page) string {
	const js = `
	() => {
	  try {
	    const el = document.getElementById('__NEXT_DATA__');
	    if (!el) return '';
	    const data = JSON.parse(el.textContent);
	    const pp = (data && data.props && data.props.pageProps) || {};
	    const s = pp.session || {};
	    return s?.user?.email || s?.email || pp?.user?.email || '';
	  } catch { return ''; }
	}
	`
	res, err := p.Eval(js)
	if err != nil || res == nil {
		return ""
	}
	var out string
	if err := res.Value.Unmarshal(&out); err != nil {
		return ""
	}
	return out
}

// profileSize walks userDataDir and sums file sizes. Capped at 2 seconds so
// the UI never freezes on huge profiles.
func profileSize(dir string) int64 {
	if dir == "" {
		return 0
	}
	deadline := time.Now().Add(2 * time.Second)
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if time.Now().After(deadline) {
			return filepath.SkipAll
		}
		info, err := d.Info()
		if err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
