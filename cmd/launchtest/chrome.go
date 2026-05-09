package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func detectChrome(override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
	}
	for _, p := range []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
	} {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("chrome.exe not found")
}

// probeReuse hits /json/version with up to `tries` attempts spaced by
// `interval`. Returns the wsURL on success.
func probeReuse(port, tries int, interval time.Duration) (string, bool) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	for i := 0; i < tries; i++ {
		c := http.Client{Timeout: interval}
		resp, err := c.Get(url)
		if err != nil {
			time.Sleep(interval)
			continue
		}
		var v struct {
			Browser              string `json:"Browser"`
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&v)
		_ = resp.Body.Close()
		if v.WebSocketDebuggerURL != "" {
			return v.WebSocketDebuggerURL, true
		}
	}
	return "", false
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

func launchFresh(chromePath, userDataDir string, port int) (string, int, error) {
	l := launcher.New().
		Bin(chromePath).
		Set("user-data-dir", userDataDir).
		Delete("enable-automation").
		Delete("use-mock-keychain").
		Delete("disable-features").
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-features", "AutomationControlled,Translate,OptimizationGuideModelDownloading,IsolateOrigins,site-per-process").
		Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36").
		Set("lang", "en-US,en;q=0.9").
		Set("no-first-run").
		Set("no-default-browser-check").
		Set("disable-default-apps").
		Set("password-store", "basic").
		Set("remote-debugging-port", strconv.Itoa(port)).
		Set("remote-allow-origins", "*").
		Headless(false).
		Devtools(false).
		Leakless(false)

	ws, err := l.Launch()
	if err != nil {
		return "", 0, err
	}
	return ws, l.PID(), nil
}

func ensureLabsTab(ctx context.Context, b *rod.Browser) (*rod.Page, error) {
	_ = ctx
	pages, err := b.Pages()
	if err == nil {
		for _, p := range pages {
			info, err := p.Info()
			if err != nil {
				continue
			}
			if strings.Contains(info.URL, "labs.google") {
				return p, nil
			}
		}
	}
	return b.MustPage(labsURL), nil
}
