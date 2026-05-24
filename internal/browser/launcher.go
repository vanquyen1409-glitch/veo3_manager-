package browser

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// RealisticUA is the FALLBACK stock-Chrome UA used only when we cannot detect
// the real installed Chrome version (see realisticUA). Override rod's default
// "HeadlessChrome" UA that Google's account login screen rejects with
// "This browser or app may not be secure".
//
// IMPORTANT: prefer realisticUA(chromePath) — a hard-coded version goes stale
// and a UA that lags far behind the real browser is itself a "may not be
// secure" trigger, and it desyncs from the safe-login window (which uses the
// genuine Chrome UA), invalidating the session Google just authenticated.
const RealisticUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

// chromeVersionDir matches Chrome's versioned sub-folder name, e.g. "148.0.7778.179".
var chromeVersionDir = regexp.MustCompile(`^(\d+)\.\d+\.\d+\.\d+$`)

// realisticUA returns a stock Chrome UA whose major version matches the REAL
// Chrome binary at chromePath. Chrome installs a versioned sub-folder next to
// chrome.exe (…\Application\148.0.7778.179\); we read the major from there.
//
// Matching the installed version is what keeps the CDP session in lock-step
// with the safe-login window: both present the same UA, so the cookies Google
// minted in one are accepted in the other. Falls back to RealisticUA if the
// folder layout is unexpected.
func realisticUA(chromePath string) string {
	if major := detectChromeMajor(chromePath); major != "" {
		return fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", major)
	}
	return RealisticUA
}

// detectChromeMajor reads the major version from Chrome's versioned sub-folder
// beside chrome.exe. Returns "" when it cannot be determined.
func detectChromeMajor(chromePath string) string {
	if chromePath == "" {
		return ""
	}
	entries, err := os.ReadDir(filepath.Dir(chromePath))
	if err != nil {
		return ""
	}
	best := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m := chromeVersionDir.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		// Multiple versions can coexist after an update; pick the highest.
		if best == "" || lessVersion(best, m[1]) {
			best = m[1]
		}
	}
	return best
}

// lessVersion reports whether major a < b for numeric-string majors.
func lessVersion(a, b string) bool {
	ai, _ := strconv.Atoi(a)
	bi, _ := strconv.Atoi(b)
	return ai < bi
}

// detectChrome returns the first existing Chrome binary path on Windows.
// If override is non-empty and exists, it wins.
func detectChrome(override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", ErrChromeNotFound
}

// jsonVersion is the response shape of /json/version on a CDP-enabled Chrome.
type jsonVersion struct {
	Browser              string `json:"Browser"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// tryReuse pings localhost:<port>/json/version. On success it returns a connected
// rod.Browser plus the WebSocket debugger URL. The URL is exposed to the UI so
// users can paste it into external automation tools.
func tryReuse(port int) (*rod.Browser, string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	c := http.Client{Timeout: 700 * time.Millisecond}
	resp, err := c.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var v jsonVersion
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, "", err
	}
	if v.WebSocketDebuggerURL == "" {
		return nil, "", fmt.Errorf("no webSocketDebuggerUrl")
	}
	b := rod.New().ControlURL(v.WebSocketDebuggerURL)
	if err := b.Connect(); err != nil {
		return nil, "", err
	}
	if _, err := b.Version(); err != nil {
		_ = b.Close()
		return nil, "", err
	}
	return b, v.WebSocketDebuggerURL, nil
}

// launchFresh starts a new Chrome with persistent profile, stealth flags, and
// a pinned remote debugging port. Returns the connected browser, OS PID, and
// the WebSocket debugger URL (so the UI can show it for external tools).
//
// Anti-detection: rod's default flags include `--enable-automation` and
// `--use-mock-keychain` — both signal the browser is being driven. We delete
// those, override the UA, and add CDP-channel hides.
func launchFresh(chromePath, userDataDir string, port int) (*rod.Browser, int, string, error) {
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return nil, 0, "", err
	}
	if !portFree(port) {
		return nil, 0, "", fmt.Errorf("debug port %d is in use", port)
	}

	slog.Info("launchFresh start", "chromePath", chromePath, "userDataDir", userDataDir, "port", port)

	l := launcher.New().
		Bin(chromePath).
		Set("user-data-dir", userDataDir).
		// Remove rod defaults that scream "automation" to anti-bot scripts.
		Delete("enable-automation").
		Delete("use-mock-keychain").
		// Override default rod feature blocklist that contains automation hints.
		Delete("disable-features").
		// Realistic, full-featured Chrome.
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-features", "AutomationControlled,Translate,OptimizationGuideModelDownloading,IsolateOrigins,site-per-process").
		Set("enable-features", "NetworkService,NetworkServiceInProcess").
		Set("user-agent", realisticUA(chromePath)).
		Set("lang", "en-US,en;q=0.9").
		Set("disable-component-extensions-with-background-pages").
		Set("no-first-run").
		Set("no-default-browser-check").
		Set("disable-default-apps").
		Set("password-store", "basic").
		// Open the Chrome window maximized so the Veo/Flow UI fills the screen.
		// Without this rod opens Chrome at its small default size and the labs
		// page renders in a cramped viewport. CDP also re-asserts this after
		// connect (maximizeWindow) so reused windows get fixed too.
		Set("start-maximized").
		Set("remote-debugging-port", strconv.Itoa(port)).
		// Restrict CDP WebSocket to localhost-origin only. With "*" any web
		// page on this machine (in any other browser profile) could connect
		// to port 9222 and exfiltrate the labs.google Bearer token. rod's
		// own connection sends Origin: http://127.0.0.1, so this still works
		// for our process while blocking external/cross-origin pages and
		// DNS-rebinding attacks (those keep their original Origin).
		Set("remote-allow-origins", "http://127.0.0.1").
		Headless(false).
		Devtools(false).
		Leakless(false)

	slog.Info("calling launcher.Launch")
	wsURL, err := l.Launch()
	if err != nil {
		slog.Error("launcher.Launch failed", "err", err)
		return nil, 0, "", fmt.Errorf("launch: %w", err)
	}
	slog.Info("got wsURL", "ws", wsURL)
	b := rod.New().ControlURL(wsURL)
	if err := b.Connect(); err != nil {
		slog.Error("rod connect failed", "err", err)
		return nil, 0, "", fmt.Errorf("connect: %w", err)
	}
	slog.Info("rod connected")
	pid := l.PID()
	return b, pid, wsURL, nil
}

// launchSafeLogin spawns Chrome WITHOUT a remote debugging port — i.e. a
// "vanilla" Chrome that Google cannot detect as automated. Used as the
// dedicated "Đăng nhập an toàn" path: user signs in to Gmail in this window,
// closes Chrome, and the persistent profile retains cookies. The next normal
// CDP launch reads those cookies and skips the login wall.
//
// Returns the OS process so caller can detect when the user closes the window.
func launchSafeLogin(chromePath, userDataDir, openURL string) (*exec.Cmd, error) {
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return nil, err
	}
	if openURL == "" {
		openURL = "https://accounts.google.com/"
	}
	args := []string{
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		// NOTE: deliberately NO --remote-debugging-port and NO automation flags.
		openURL,
	}
	cmd := exec.Command(chromePath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("safe-login launch: %w", err)
	}
	slog.Info("safeLogin started", "pid", cmd.Process.Pid, "url", openURL)
	return cmd, nil
}

// maximizeWindow makes the Flow/Veo UI fill the screen. Two distinct fixes:
//
//  1. SetViewport(nil) clears rod's default device emulation. rod emulates
//     devices.LaptopWithMDPIScreen.Landscape() on every page it creates, which
//     pins the page to a fixed ~1280px viewport — the content renders small
//     with black bars on the right even when the OS window is huge. Clearing
//     the device-metrics override lets the page reflow to the real window size.
//     (We also call NoDefaultDevice() at connect time so freshly-created pages
//     never get the override; this handles reused tabs that already have it.)
//
//  2. SetWindow(maximized) maximizes the OS window itself. Complements the
//     --start-maximized launch flag and covers the reuse path (tryReuse), where
//     we attach to a Chrome that was started earlier at its default small size.
//
// Best-effort: a failure here only means a non-maximized window, never a
// connect failure. Only WindowState is set (no width/height) — Chrome rejects
// bounds changes unless the state is "normal", but a lone windowState
// transition is always allowed.
func maximizeWindow(page *rod.Page) error {
	_ = page.SetViewport(nil)
	return page.SetWindow(&proto.BrowserBounds{WindowState: proto.BrowserWindowStateMaximized})
}

func portFree(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return true
	}
	_ = c.Close()
	return false
}
