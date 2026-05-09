package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"veo3-manager/internal/db"
	"veo3-manager/internal/download"
	"veo3-manager/internal/labsapi"
)

// maxPollRetries is how many times we extend the poll deadline before failing
// the task. Veo3 sometimes queues jobs for 8-10 min during peak load while the
// configured timeout is typically 5 min. With base 5 min + 2 retries doubling
// each time we wait up to 5 + 10 + 20 = 35 min before giving up. This avoids
// false "failed" tasks while still preventing infinite stalls.
const maxPollRetries = 2

// runTask runs a single task end-to-end. Errors mark task failed but do not
// crash the loop.
func (w *Worker) runTask(ctx context.Context, t *db.Task, bundle db.SettingsBundle) {
	w.emitTask(t.ID, "running", map[string]any{"prompt": t.Prompt})

	cfg := normalizeGenerationConfig(t.Config, bundle.OutputDir)
	subCfg := buildSubmitConfig(cfg)

	refs, err := w.deps.API.Submit(ctx, t.Prompt, subCfg)
	if err != nil {
		w.failTask(t.ID, fmt.Errorf("submit: %w", err))
		return
	}

	// Submit can flip the Flow UI into /edit/<workflow> mode, which hides
	// the prompt input + sidebar. Pop back to the project root so the
	// user keeps seeing the normal Flow UI.
	if w.deps.Browser != nil {
		_ = w.deps.Browser.EnsureProjectView()
	}

	mediaIDs := make([]string, 0, len(refs))
	for _, r := range refs {
		mediaIDs = append(mediaIDs, r.MediaID)
	}
	_ = w.deps.Tasks.SaveMediaIDs(t.ID, mediaIDs)
	w.emitTask(t.ID, "submitted", map[string]any{"count": len(refs)})

	baseTimeout := time.Duration(bundle.PollTimeoutMs) * time.Millisecond
	pollOpts := labsapi.PollOptions{
		Interval: time.Duration(bundle.PollIntervalMs) * time.Millisecond,
		Timeout:  baseTimeout,
	}

	w.emitTask(t.ID, "polling", map[string]any{"i": 0, "n": len(refs)})
	finals, err := w.deps.API.Wait(ctx, refs, pollOpts)
	// Retry on poll timeout only — keep network/parse errors fatal so we
	// don't mask real bugs. ctx cancellation flows through ctx.Err() and
	// must NOT be retried (user clicked Cancel).
	for attempt := 1; err != nil && attempt <= maxPollRetries; attempt++ {
		if ctx.Err() != nil || !isPollTimeout(err) {
			break
		}
		// Exponential extension: 2x, 4x base.
		pollOpts.Timeout = baseTimeout << attempt
		w.deps.Logger.Warn("poll timed out, retrying with extended deadline",
			"task", t.ID, "attempt", attempt, "newTimeout", pollOpts.Timeout)
		w.emitTask(t.ID, "polling", map[string]any{
			"i": 0, "n": len(refs),
			"retry":   attempt,
			"timeout": pollOpts.Timeout.String(),
		})
		finals, err = w.deps.API.Wait(ctx, refs, pollOpts)
	}
	if err != nil {
		w.failTask(t.ID, fmt.Errorf("poll: %w", err))
		return
	}

	savedPaths := w.downloadFinals(ctx, t.ID, cfg.OutputDir, finals)
	if savedPaths == nil {
		// ctx cancelled mid-download; cancelTask already emitted.
		return
	}
	if len(savedPaths) == 0 {
		w.failTask(t.ID, errors.New("all outputs failed"))
		return
	}

	if err := w.deps.Tasks.UpdateStatus(t.ID, db.StatusSucceeded, ""); err != nil {
		w.deps.Logger.Error("update status succeeded", "err", err)
	}
	w.emitTask(t.ID, "succeeded", map[string]any{"paths": savedPaths})
}

// downloadFinals fetches each successful media and returns the saved paths.
// Returns nil when ctx is cancelled mid-flight (cancelTask already emitted).
func (w *Worker) downloadFinals(ctx context.Context, taskID, outputDir string, finals []labsapi.FinalMedia) []string {
	saved := make([]string, 0, len(finals))
	for i, fin := range finals {
		select {
		case <-ctx.Done():
			w.cancelTask(taskID)
			return nil
		default:
		}
		if fin.Status != labsapi.StatusSuccessful {
			w.deps.Logger.Warn("media not successful", "task", taskID, "media", fin.MediaID, "status", string(fin.Status))
			continue
		}

		dst := download.BuildPath(outputDir, taskID, fin.Seed, "mp4")
		w.emitTask(taskID, "downloading", map[string]any{"i": i + 1, "n": len(finals)})

		// Build the labs.google redirect URL from the media id; the
		// downloader resolves it to a flow-content.google signed URL.
		redirectURL := fmt.Sprintf(labsapi.MediaRedirectURLFmt, fin.MediaID)
		if err := w.deps.Download.Fetch(ctx, redirectURL, dst, nil); err != nil {
			w.deps.Logger.Warn("download failed", "task", taskID, "media", fin.MediaID, "err", err)
			continue
		}
		_ = w.deps.Tasks.AppendOutput(taskID, dst)
		saved = append(saved, dst)
	}
	return saved
}

// normalizeGenerationConfig fills defaults and clamps user-supplied config
// before submitting to the labs API.
func normalizeGenerationConfig(cfg db.GenerationConfig, defaultOutputDir string) db.GenerationConfig {
	if cfg.OutputDir == "" {
		cfg.OutputDir = defaultOutputDir
	}
	// Coerce stale model id from earlier builds to the actual working one.
	if cfg.Model == "" || cfg.Model == "veo_3_1_t2v_fast_ultra" {
		cfg.Model = "veo_3_1_t2v_fast"
	}
	// Veo only supports 16:9 + 9:16 — fall back to landscape on anything else.
	if cfg.AspectRatio != "16:9" && cfg.AspectRatio != "9:16" {
		cfg.AspectRatio = "16:9"
	}
	if cfg.OutputCount < 1 {
		cfg.OutputCount = 1
	}
	if cfg.OutputCount > 4 {
		cfg.OutputCount = 4
	}
	return cfg
}

func buildSubmitConfig(cfg db.GenerationConfig) labsapi.SubmitConfig {
	sub := labsapi.SubmitConfig{
		Model:       cfg.Model,
		AspectRatio: cfg.AspectRatio,
		OutputCount: cfg.OutputCount,
	}
	if cfg.SeedBase > 0 {
		sub.Seeds = make([]int64, cfg.OutputCount)
		for i := range sub.Seeds {
			sub.Seeds[i] = cfg.SeedBase + int64(i)
		}
	}
	return sub
}

// isPollTimeout returns true when the error came from labsapi.Wait reaching
// its deadline. labsapi formats it as "poll timeout after <duration>". We
// match by substring rather than a sentinel because the labs package builds
// it via fmt.Errorf without a wrapped sentinel.
func isPollTimeout(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "poll timeout")
}

func (w *Worker) failTask(id string, err error) {
	msg := err.Error()
	_ = w.deps.Tasks.UpdateStatus(id, db.StatusFailed, msg)
	w.emitTask(id, "failed", map[string]any{"error": msg})
}

func (w *Worker) cancelTask(id string) {
	_ = w.deps.Tasks.UpdateStatus(id, db.StatusCancelled, "cancelled by user")
	w.emitTask(id, "cancelled", nil)
}
