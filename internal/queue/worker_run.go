package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
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

// minDiskFreeBytes is the floor we require in the output dir before starting
// downloads. 4× outputs × ~50 MB Veo3 8s clip + slack = 500 MB. Below this we
// fail the task fast with a clear message instead of letting a partial write
// hit ENOSPC mid-stream and leave a corrupt file behind.
const minDiskFreeBytes = 500 * 1024 * 1024

// runTask runs a single task end-to-end. Errors mark task failed but do not
// crash the loop.
//
// All log lines downstream are tagged with `task=<id>` via the scoped logger
// `tlog`. Use `tlog` (not w.deps.Logger directly) anywhere we have a task
// in hand so log filtering by task ID is reliable.
func (w *Worker) runTask(ctx context.Context, t *db.Task, bundle db.SettingsBundle) {
	tlog := w.deps.Logger.With("task", t.ID, "attempt", t.Attempts)
	tlog.Info("task start", "prompt_len", len(t.Prompt))
	w.emitTask(t.ID, "running", map[string]any{"prompt": t.Prompt})

	cfg := normalizeGenerationConfig(t.Config, bundle.OutputDir)
	subCfg := buildSubmitConfig(cfg)

	refs, err := w.deps.API.Submit(ctx, t.Prompt, subCfg)
	if err != nil {
		tlog.Warn("submit failed", "err", err)
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
	for retry := 1; err != nil && retry <= maxPollRetries; retry++ {
		if ctx.Err() != nil || !isPollTimeout(err) {
			break
		}
		// Exponential extension: 2x, 4x base.
		pollOpts.Timeout = baseTimeout << retry
		tlog.Warn("poll timed out, retrying with extended deadline",
			"retry", retry, "newTimeout", pollOpts.Timeout)
		w.emitTask(t.ID, "polling", map[string]any{
			"i": 0, "n": len(refs),
			"retry":   retry,
			"timeout": pollOpts.Timeout.String(),
		})
		finals, err = w.deps.API.Wait(ctx, refs, pollOpts)
	}
	if err != nil {
		tlog.Warn("poll failed", "err", err)
		w.failTask(t.ID, fmt.Errorf("poll: %w", err))
		return
	}

	// Disk-space precheck: refuse to start downloads if we can't even fit
	// the next batch. Better a clear "low disk" error than a corrupt video.
	if free, derr := download.DiskFreeBytes(cfg.OutputDir); derr == nil && free > 0 && free < minDiskFreeBytes {
		tlog.Warn("disk space too low", "free_mb", free/1024/1024, "required_mb", minDiskFreeBytes/1024/1024, "dir", cfg.OutputDir)
		w.failTask(t.ID, fmt.Errorf("disk space too low: %d MB free at %s, need >= %d MB",
			free/1024/1024, cfg.OutputDir, minDiskFreeBytes/1024/1024))
		return
	}

	savedPaths := w.downloadFinals(ctx, tlog, t.ID, cfg.OutputDir, finals, bundle.MaxParallelDl)
	if savedPaths == nil {
		// ctx cancelled mid-download; cancelTask already emitted.
		return
	}
	if len(savedPaths) == 0 {
		tlog.Warn("all outputs failed to download")
		w.failTask(t.ID, errors.New("all outputs failed"))
		return
	}

	if err := w.deps.Tasks.UpdateStatus(t.ID, db.StatusSucceeded, ""); err != nil {
		tlog.Error("update status succeeded", "err", err)
	}
	tlog.Info("task succeeded", "saved_count", len(savedPaths))
	w.emitTask(t.ID, "succeeded", map[string]any{"paths": savedPaths})
}

// downloadFinals fetches successful media in parallel (bounded by parallelDl)
// and returns the saved paths in their original order. Returns nil when ctx
// is cancelled mid-flight (cancelTask already emitted).
//
// parallelDl is clamped to [1, 8] - 8 is plenty for a single Veo3 batch and
// avoids saturating the user's network. Veo3 batches are 1-4 outputs in
// practice, so this only matters when the setting is wrong.
//
// Takes a scoped logger (tlog) so every line is tagged with task=<id>.
func (w *Worker) downloadFinals(
	ctx context.Context,
	tlog *slog.Logger,
	taskID, outputDir string,
	finals []labsapi.FinalMedia,
	parallelDl int,
) []string {
	if parallelDl < 1 {
		parallelDl = 1
	}
	if parallelDl > 8 {
		parallelDl = 8
	}

	// saved is keyed by original index so order is preserved on success.
	var (
		mu        sync.Mutex
		saved     = make(map[int]string, len(finals))
		completed atomic.Int32
		sem       = make(chan struct{}, parallelDl)
		wg        sync.WaitGroup
	)

	total := 0
	for _, fin := range finals {
		if fin.Status == labsapi.StatusSuccessful {
			total++
		}
	}

	for i, fin := range finals {
		// Honour cancellation on the dispatch side: bail before spawning
		// new goroutines once the user clicked Cancel.
		if ctx.Err() != nil {
			break
		}
		if fin.Status != labsapi.StatusSuccessful {
			tlog.Warn("media not successful",
				"media", fin.MediaID, "status", string(fin.Status))
			continue
		}

		wg.Add(1)
		sem <- struct{}{} // blocks until a slot is free
		go func(i int, fin labsapi.FinalMedia) {
			defer wg.Done()
			defer func() { <-sem }()

			dst := download.BuildPath(outputDir, taskID, fin.Seed, "mp4")
			redirectURL := fmt.Sprintf(labsapi.MediaRedirectURLFmt, fin.MediaID)
			if err := w.deps.Download.Fetch(ctx, redirectURL, dst, nil); err != nil {
				tlog.Warn("download failed", "media", fin.MediaID, "err", err)
				return
			}
			_ = w.deps.Tasks.AppendOutput(taskID, dst)

			n := int(completed.Add(1))
			w.emitTask(taskID, "downloading", map[string]any{
				"i": n, "n": total, "parallel": parallelDl,
			})

			mu.Lock()
			saved[i] = dst
			mu.Unlock()
		}(i, fin)
	}
	wg.Wait()

	if ctx.Err() != nil {
		w.cancelTask(taskID)
		return nil
	}

	// Preserve original Veo3 batch order.
	ordered := make([]string, 0, len(saved))
	for i := 0; i < len(finals); i++ {
		if p, ok := saved[i]; ok {
			ordered = append(ordered, p)
		}
	}
	return ordered
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
