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
	"strconv"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// RealisticUA mimics a stock Chrome on Windows. Used to override rod's default
// "HeadlessChrome" UA that Google's account login screen rejects with
// "This browser or app may not be secure".
const RealisticUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

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
		Set("user-agent", RealisticUA).
		Set("lang", "en-US,en;q=0.9").
		Set("disable-component-extensions-with-background-pages").
		Set("no-first-run").
		Set("no-default-browser-check").
		Set("disable-default-apps").
		Set("password-store", "basic").
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

func portFree(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return true
	}
	_ = c.Close()
	return false
}
