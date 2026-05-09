package queue

import (
	"errors"
	"testing"

	"veo3-manager/internal/db"
)

func TestNormalizeGenerationConfig(t *testing.T) {
	const defaultDir = "C:/default/out"

	tests := []struct {
		name string
		in   db.GenerationConfig
		want db.GenerationConfig
	}{
		{
			name: "empty cfg fills every default",
			in:   db.GenerationConfig{},
			want: db.GenerationConfig{
				Model:       "veo_3_1_t2v_fast",
				AspectRatio: "16:9",
				OutputCount: 1,
				OutputDir:   defaultDir,
			},
		},
		{
			name: "deprecated ultra model coerced to fast",
			in:   db.GenerationConfig{Model: "veo_3_1_t2v_fast_ultra", AspectRatio: "16:9", OutputCount: 1, OutputDir: "x"},
			want: db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 1, OutputDir: "x"},
		},
		{
			name: "invalid aspect ratio falls back to landscape",
			in:   db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "1:1", OutputCount: 2, OutputDir: "x"},
			want: db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 2, OutputDir: "x"},
		},
		{
			name: "valid 9:16 portrait passes through",
			in:   db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "9:16", OutputCount: 1, OutputDir: "x"},
			want: db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "9:16", OutputCount: 1, OutputDir: "x"},
		},
		{
			name: "OutputCount 0 clamps up to 1",
			in:   db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 0, OutputDir: "x"},
			want: db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 1, OutputDir: "x"},
		},
		{
			name: "OutputCount negative clamps up to 1",
			in:   db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: -5, OutputDir: "x"},
			want: db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 1, OutputDir: "x"},
		},
		{
			name: "OutputCount 99 clamps down to 4",
			in:   db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 99, OutputDir: "x"},
			want: db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 4, OutputDir: "x"},
		},
		{
			name: "user OutputDir wins over default",
			in:   db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 1, OutputDir: "D:/custom"},
			want: db.GenerationConfig{Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 1, OutputDir: "D:/custom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGenerationConfig(tt.in, defaultDir)
			if got != tt.want {
				t.Errorf("normalize() = %+v\n            want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildSubmitConfig_SeedExpansion(t *testing.T) {
	cfg := db.GenerationConfig{
		Model:       "veo_3_1_t2v_fast",
		AspectRatio: "16:9",
		OutputCount: 3,
		SeedBase:    1000,
	}
	got := buildSubmitConfig(cfg)
	if got.OutputCount != 3 {
		t.Fatalf("OutputCount = %d, want 3", got.OutputCount)
	}
	if len(got.Seeds) != 3 {
		t.Fatalf("len(Seeds) = %d, want 3", len(got.Seeds))
	}
	want := []int64{1000, 1001, 1002}
	for i, s := range got.Seeds {
		if s != want[i] {
			t.Errorf("Seeds[%d] = %d, want %d", i, s, want[i])
		}
	}
}

func TestBuildSubmitConfig_NoSeedBase(t *testing.T) {
	cfg := db.GenerationConfig{
		Model:       "veo_3_1_t2v_fast",
		AspectRatio: "16:9",
		OutputCount: 2,
		SeedBase:    0,
	}
	got := buildSubmitConfig(cfg)
	if got.Seeds != nil {
		t.Errorf("Seeds = %v, want nil when SeedBase=0 (API picks random)", got.Seeds)
	}
}

func TestIsPollTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error is not timeout", nil, false},
		{"exact match from labsapi", errors.New("poll timeout after 5m0s"), true},
		{"wrapped poll timeout matches", errors.New("submit: poll timeout after 10m0s: gave up"), true},
		{"unrelated error", errors.New("network unreachable"), false},
		{"context cancelled is NOT a poll timeout (must not be retried)", errors.New("context canceled"), false},
		{"context deadline is NOT a poll timeout", errors.New("context deadline exceeded"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPollTimeout(tt.err); got != tt.want {
				t.Errorf("isPollTimeout(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
