package db

import "testing"

func TestApplyKV(t *testing.T) {
	t.Run("non-empty paths overwrite defaults", func(t *testing.T) {
		b := SettingsBundle{
			UserDataDir:    "/default/data",
			OutputDir:      "/default/out",
			ChromeDebugPort: 9222,
		}
		applyKV(&b, "userDataDir", "/custom/data")
		applyKV(&b, "outputDir", "/custom/out")
		if b.UserDataDir != "/custom/data" {
			t.Errorf("UserDataDir = %q, want /custom/data", b.UserDataDir)
		}
		if b.OutputDir != "/custom/out" {
			t.Errorf("OutputDir = %q, want /custom/out", b.OutputDir)
		}
	})

	t.Run("empty path values do NOT overwrite defaults", func(t *testing.T) {
		// Critical: a stored empty string must not nuke the platform default
		// path (otherwise on macOS/Linux a fresh-install user with stale Win
		// settings rows could land in cwd).
		b := SettingsBundle{UserDataDir: "/default/data", OutputDir: "/default/out"}
		applyKV(&b, "userDataDir", "")
		applyKV(&b, "outputDir", "")
		if b.UserDataDir != "/default/data" {
			t.Errorf("empty userDataDir overwrote default: got %q", b.UserDataDir)
		}
		if b.OutputDir != "/default/out" {
			t.Errorf("empty outputDir overwrote default: got %q", b.OutputDir)
		}
	})

	t.Run("chromePath always overwrites (allowed to be empty)", func(t *testing.T) {
		// chromePath="" means "auto-detect", so empty MUST overwrite a stale
		// hardcoded path.
		b := SettingsBundle{ChromePath: "/old/chrome"}
		applyKV(&b, "chromePath", "")
		if b.ChromePath != "" {
			t.Errorf("chromePath = %q, want empty", b.ChromePath)
		}
	})

	t.Run("integer fields parse JSON-encoded ints", func(t *testing.T) {
		b := SettingsBundle{ChromeDebugPort: 9222, PollIntervalMs: 10000, PollTimeoutMs: 300000, MaxParallelDl: 1}
		applyKV(&b, "chromeDebugPort", "9333")
		applyKV(&b, "pollIntervalMs", "5000")
		applyKV(&b, "pollTimeoutMs", "600000")
		applyKV(&b, "maxParallelDl", "3")
		if b.ChromeDebugPort != 9333 {
			t.Errorf("ChromeDebugPort = %d, want 9333", b.ChromeDebugPort)
		}
		if b.PollIntervalMs != 5000 {
			t.Errorf("PollIntervalMs = %d, want 5000", b.PollIntervalMs)
		}
		if b.PollTimeoutMs != 600000 {
			t.Errorf("PollTimeoutMs = %d, want 600000", b.PollTimeoutMs)
		}
		if b.MaxParallelDl != 3 {
			t.Errorf("MaxParallelDl = %d, want 3", b.MaxParallelDl)
		}
	})

	t.Run("zero or negative integer values are rejected", func(t *testing.T) {
		// Negatives + zero would cause infinite tight-loop polls or division
		// by zero. Defaults must survive corrupt rows.
		b := SettingsBundle{ChromeDebugPort: 9222, PollIntervalMs: 10000, PollTimeoutMs: 300000, MaxParallelDl: 1}
		applyKV(&b, "chromeDebugPort", "0")
		applyKV(&b, "pollIntervalMs", "-100")
		applyKV(&b, "pollTimeoutMs", "0")
		applyKV(&b, "maxParallelDl", "-5")
		if b.ChromeDebugPort != 9222 {
			t.Errorf("ChromeDebugPort = %d, want 9222 unchanged", b.ChromeDebugPort)
		}
		if b.PollIntervalMs != 10000 {
			t.Errorf("PollIntervalMs = %d, want 10000 unchanged", b.PollIntervalMs)
		}
		if b.PollTimeoutMs != 300000 {
			t.Errorf("PollTimeoutMs = %d, want 300000 unchanged", b.PollTimeoutMs)
		}
		if b.MaxParallelDl != 1 {
			t.Errorf("MaxParallelDl = %d, want 1 unchanged", b.MaxParallelDl)
		}
	})

	t.Run("invalid JSON int leaves field unchanged", func(t *testing.T) {
		b := SettingsBundle{ChromeDebugPort: 9222}
		applyKV(&b, "chromeDebugPort", "not-a-number")
		if b.ChromeDebugPort != 9222 {
			t.Errorf("garbage value mutated field: %d", b.ChromeDebugPort)
		}
	})

	t.Run("defaultConfig parses JSON object", func(t *testing.T) {
		b := SettingsBundle{}
		applyKV(&b, "defaultConfig", `{"model":"veo_3_1_t2v_fast","aspectRatio":"9:16","outputCount":2,"seedBase":42,"outputDir":"/x"}`)
		if b.DefaultConfig.Model != "veo_3_1_t2v_fast" {
			t.Errorf("Model = %q", b.DefaultConfig.Model)
		}
		if b.DefaultConfig.AspectRatio != "9:16" {
			t.Errorf("AspectRatio = %q", b.DefaultConfig.AspectRatio)
		}
		if b.DefaultConfig.OutputCount != 2 {
			t.Errorf("OutputCount = %d", b.DefaultConfig.OutputCount)
		}
		if b.DefaultConfig.SeedBase != 42 {
			t.Errorf("SeedBase = %d", b.DefaultConfig.SeedBase)
		}
	})

	t.Run("malformed defaultConfig JSON leaves field unchanged", func(t *testing.T) {
		b := SettingsBundle{DefaultConfig: GenerationConfig{Model: "keep"}}
		applyKV(&b, "defaultConfig", `{"this is not json`)
		if b.DefaultConfig.Model != "keep" {
			t.Errorf("malformed JSON corrupted bundle: %+v", b.DefaultConfig)
		}
	})

	t.Run("unknown key is silently ignored", func(t *testing.T) {
		b := SettingsBundle{ChromePath: "/x"}
		applyKV(&b, "totallyUnknown", "anything")
		if b.ChromePath != "/x" {
			t.Errorf("unknown key mutated bundle: %+v", b)
		}
	})
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"0", 0, true},
		{"42", 42, true},
		{"-1", -1, true},
		{"", 0, false},
		{"abc", 0, false},
		{"1.5", 0, false},
		{"  3  ", 3, true}, // json.Unmarshal accepts surrounding whitespace per RFC 7159
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := parseInt(tt.in)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("value = %d, want %d", got, tt.want)
			}
		})
	}
}
