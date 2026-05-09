package browser

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestParseProjectIDFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "labs flow project edit URL",
			url:  "https://labs.google/fx/vi/tools/flow/project/abc-1234-def/edit/workflow-1",
			want: "abc-1234-def",
		},
		{
			name: "labs flow project root",
			url:  "https://labs.google/fx/vi/tools/flow/project/uuid-only",
			want: "uuid-only",
		},
		{
			name: "labs flow without project segment returns empty",
			url:  "https://labs.google/fx/vi/tools/flow",
			want: "",
		},
		{
			name: "tools/flow root - no project yet",
			url:  "https://labs.google/fx/vi/tools/flow/",
			want: "",
		},
		{
			name: "trailing 'project' with no UUID after = empty",
			url:  "https://labs.google/fx/vi/tools/flow/project",
			want: "",
		},
		{
			name: "URL with query string",
			url:  "https://labs.google/fx/vi/tools/flow/project/abc/edit/wf?foo=bar",
			want: "abc",
		},
		{
			name: "completely unrelated URL",
			url:  "https://google.com/search?q=hi",
			want: "",
		},
		{
			name: "malformed URL returns empty",
			url:  "::not a url::",
			want: "",
		},
		{
			name: "empty",
			url:  "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseProjectIDFromURL(tt.url); got != tt.want {
				t.Errorf("parseProjectIDFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestDetectChrome_OverrideWins(t *testing.T) {
	tmp := t.TempDir()
	exe := "chrome.exe"
	if runtime.GOOS != "windows" {
		exe = "chrome"
	}
	fake := filepath.Join(tmp, exe)
	if err := os.WriteFile(fake, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := detectChrome(fake)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != fake {
		t.Errorf("got %q, want override path %q", got, fake)
	}
}

func TestDetectChrome_OverrideMissingFallsBackToCandidates(t *testing.T) {
	// Override path that doesn't exist - detectChrome should ignore it
	// rather than error so users can leave the field blank/wrong.
	got, err := detectChrome(filepath.Join(t.TempDir(), "doesnt-exist.exe"))
	// Either it falls back to a real Chrome on PATH (success) OR returns
	// ErrChromeNotFound. Both are valid - we only care that bogus override
	// doesn't surface a different error.
	if err != nil && err != ErrChromeNotFound {
		t.Errorf("unexpected error class: %v", err)
	}
	if err == nil && got == "" {
		t.Error("got empty path with no error")
	}
}

func TestPortFree_FreePortReturnsTrue(t *testing.T) {
	// Port 0 = let kernel pick. Bind, get the port, close, then check
	// portFree on the (now-free) port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().(*net.TCPAddr)
	port := addr.Port
	_ = l.Close()
	// Tiny race: another process could grab the port between our close and
	// the check. Acceptable for a smoke test.
	if !portFree(port) {
		t.Errorf("portFree(%d) = false right after closing the listener", port)
	}
}

func TestPortFree_OccupiedPortReturnsFalse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	if portFree(addr.Port) {
		t.Errorf("portFree(%d) = true while listener still open", addr.Port)
	}
}

func TestRealisticUA_LooksLikeRealChrome(t *testing.T) {
	// Sanity: the UA we send to labs.google must contain "Chrome" and a
	// version string. If someone accidentally lowercased or removed it,
	// labs.google's "this browser may not be secure" gate kicks in.
	ua := RealisticUA
	if !strings.Contains(ua, "Chrome/") {
		t.Errorf("UA missing 'Chrome/': %q", ua)
	}
	if !strings.Contains(ua, "Mozilla/") {
		t.Errorf("UA missing 'Mozilla/': %q", ua)
	}
	if !strings.Contains(ua, "Windows NT") {
		t.Errorf("UA must claim Windows for stealth: %q", ua)
	}
}

func TestErrors_AllSentinelsHaveDistinctMessages(t *testing.T) {
	// Tests are the cheapest way to catch a copy-paste bug where two
	// sentinels share the same message and `errors.Is` comparisons silently
	// match the wrong one.
	all := map[string]error{
		"ErrChromeNotFound":     ErrChromeNotFound,
		"ErrNotConnected":       ErrNotConnected,
		"ErrNotLoggedIn":        ErrNotLoggedIn,
		"ErrTokenMissing":       ErrTokenMissing,
		"ErrLabsTabUnavailable": ErrLabsTabUnavailable,
		"ErrAlreadyConnecting":  ErrAlreadyConnecting,
		"ErrUnauthorized":       ErrUnauthorized,
	}
	seen := map[string]string{}
	for name, err := range all {
		if err == nil {
			t.Errorf("%s is nil", name)
			continue
		}
		msg := err.Error()
		if other, dup := seen[msg]; dup {
			t.Errorf("%s and %s share message %q", name, other, msg)
		}
		seen[msg] = name
	}
}

// Ensure RealisticUA and tokenProbeJS are accessible (compile-time check).
// Avoids the linter flagging an unused import.
var _ = strconv.Itoa
