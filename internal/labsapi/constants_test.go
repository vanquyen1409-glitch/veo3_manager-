package labsapi

import "testing"

// TestVideoModelKeyFor locks the family+aspect → videoModelKey mapping
// confirmed from the live Flow flow.projectInitialData response (2026-05-24).
func TestVideoModelKeyFor(t *testing.T) {
	tests := []struct {
		family  string
		aspect  string
		omniSec int
		want    string
	}{
		// Fast: separate landscape/portrait keys. Landscape key matches the
		// app's original known-working id.
		{"veo_3_1_fast", "16:9", 8, "veo_3_1_t2v_fast"},
		{"veo_3_1_fast", "9:16", 8, "veo_3_1_t2v_fast_portrait"},
		// Lite: one key for both orientations.
		{"veo_3_1_lite", "16:9", 8, "veo_3_1_t2v_lite"},
		{"veo_3_1_lite", "9:16", 8, "veo_3_1_t2v_lite"},
		// Quality: landscape has no suffix, portrait does.
		{"veo_3_1_quality", "16:9", 8, "veo_3_1_t2v"},
		{"veo_3_1_quality", "9:16", 8, "veo_3_1_t2v_portrait"},
		// Omni (abra): duration encoded in the key, orientation-agnostic.
		{"abra", "16:9", 4, "abra_t2v_4s"},
		{"abra", "16:9", 6, "abra_t2v_6s"},
		{"abra", "9:16", 8, "abra_t2v_8s"},
		{"abra", "16:9", 10, "abra_t2v_10s"},
		{"abra", "16:9", 7, "abra_t2v_8s"}, // unknown duration → 8s
		// Unknown family falls back to Fast landscape (the proven path).
		{"", "16:9", 8, "veo_3_1_t2v_fast"},
		{"garbage", "16:9", 8, "veo_3_1_t2v_fast"},
	}
	for _, tt := range tests {
		got := VideoModelKeyFor(tt.family, tt.aspect, tt.omniSec)
		if got != tt.want {
			t.Errorf("VideoModelKeyFor(%q, %q, %d) = %q, want %q",
				tt.family, tt.aspect, tt.omniSec, got, tt.want)
		}
	}
}
