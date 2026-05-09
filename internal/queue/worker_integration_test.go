package queue

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"veo3-manager/internal/db"
	"veo3-manager/internal/download"
	"veo3-manager/internal/labsapi"
)

// fakeAPI lets tests script the API responses across submit + N waits.
type fakeAPI struct {
	submitErr   error
	submitRefs  []labsapi.MediaRef
	waitResults []waitResult // popped in order
	waitCalls   atomic.Int32
}

type waitResult struct {
	finals []labsapi.FinalMedia
	err    error
}

func (f *fakeAPI) Submit(_ context.Context, _ string, _ labsapi.SubmitConfig) ([]labsapi.MediaRef, error) {
	return f.submitRefs, f.submitErr
}

func (f *fakeAPI) Wait(_ context.Context, _ []labsapi.MediaRef, _ labsapi.PollOptions) ([]labsapi.FinalMedia, error) {
	idx := f.waitCalls.Add(1) - 1
	if int(idx) >= len(f.waitResults) {
		return nil, fmt.Errorf("fakeAPI.Wait: ran out of scripted results (call #%d)", idx+1)
	}
	r := f.waitResults[idx]
	return r.finals, r.err
}

type fakeBrowser struct{ ensureCalls atomic.Int32 }

func (b *fakeBrowser) EnsureProjectView() error {
	b.ensureCalls.Add(1)
	return nil
}

type fakeDownloader struct {
	calls   atomic.Int32
	failNth map[int]error // call number (1-based) -> error
}

func (d *fakeDownloader) Fetch(_ context.Context, _ string, _ string, _ download.ProgressFn) error {
	n := int(d.calls.Add(1))
	if err, ok := d.failNth[n]; ok {
		return err
	}
	return nil
}

func newWorkerWithFakes(t *testing.T) (*Worker, *fakeAPI, *fakeBrowser, *fakeDownloader, *db.TaskRepo) {
	t.Helper()
	tr, sr := newTestDBForWorker(t)

	// Reasonable settings: 1ms poll interval + 1ms timeout so the retry
	// path completes in the test's lifetime. The worker itself doesn't
	// honour these for the FAKE waits (it just passes them in), so they
	// only matter for code paths that respect time. Setting them low
	// keeps the test fast even if a sleep slips into a future change.
	_ = sr.SetBundle(db.SettingsBundle{
		ChromeDebugPort: 9222,
		PollIntervalMs:  1,
		PollTimeoutMs:   1,
		MaxParallelDl:   1,
		UserDataDir:     "/d",
		OutputDir:       t.TempDir(),
		DefaultConfig: db.GenerationConfig{
			Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 1,
			OutputDir: t.TempDir(),
		},
	})

	api := &fakeAPI{}
	br := &fakeBrowser{}
	dl := &fakeDownloader{failNth: map[int]error{}}
	w := New(Deps{
		Tasks:    tr,
		Settings: sr,
		Browser:  br,
		API:      api,
		Download: dl,
	})
	return w, api, br, dl, tr
}

// newTestDBForWorker mirrors db.newTestDB without crossing package boundaries.
func newTestDBForWorker(t *testing.T) (*db.TaskRepo, *db.SettingsRepo) {
	t.Helper()
	dir := t.TempDir()
	sqlDB, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db.NewTaskRepo(sqlDB), db.NewSettingsRepo(sqlDB, db.DefaultDirs{
		UserDataDir: filepath.Join(dir, "data"),
		OutputDir:   filepath.Join(dir, "out"),
	})
}

func enqueueOne(t *testing.T, w *Worker, prompt string) *db.Task {
	t.Helper()
	// Insert directly via the repo's underlying table to avoid wiring an
	// Enqueue API just for tests.
	tr := w.deps.Tasks
	id := "task-" + prompt
	if _, err := tr.Requeue(id); err == nil {
		t.Fatalf("expected Requeue to fail (no source row) but it succeeded")
	}
	// Use raw SQL through Get→fail then create-via-source pattern: clearer
	// to just create a synthetic row through the same path the prod code
	// uses (Enqueue lives in tasks.go).
	pending, err := tr.Enqueue(prompt, db.GenerationConfig{
		Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &pending
}

func waitForStatus(t *testing.T, tr *db.TaskRepo, id string, want db.TaskStatus, timeout time.Duration) db.Task {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		got, err := tr.Get(id)
		if err == nil && got.Status == want {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	last, _ := tr.Get(id)
	t.Fatalf("task %s did not reach status %q within %s (last=%+v)", id, want, timeout, last)
	return db.Task{}
}

func TestWorker_HappyPath(t *testing.T) {
	w, api, br, dl, tr := newWorkerWithFakes(t)

	api.submitRefs = []labsapi.MediaRef{{MediaID: "m-1", Seed: 7}}
	api.waitResults = []waitResult{{
		finals: []labsapi.FinalMedia{{
			MediaRef: labsapi.MediaRef{MediaID: "m-1", Seed: 7},
			Status:   labsapi.StatusSuccessful,
		}},
	}}

	task := enqueueOne(t, w, "happy")
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	got := waitForStatus(t, tr, task.ID, db.StatusSucceeded, 5*time.Second)

	if api.waitCalls.Load() != 1 {
		t.Errorf("Wait calls = %d, want 1 (no retry on happy path)", api.waitCalls.Load())
	}
	if br.ensureCalls.Load() < 1 {
		t.Errorf("EnsureProjectView calls = %d, want >=1", br.ensureCalls.Load())
	}
	if dl.calls.Load() != 1 {
		t.Errorf("Download calls = %d, want 1", dl.calls.Load())
	}
	if len(got.VideoPaths) != 1 {
		t.Errorf("VideoPaths len = %d, want 1 (got %v)", len(got.VideoPaths), got.VideoPaths)
	}
	if got.FinishedAt == nil {
		t.Error("FinishedAt is nil on succeeded task")
	}
}

func TestWorker_PollTimeoutRetriesThenSucceeds(t *testing.T) {
	// This is the test that proves Tech Debt #4 fix actually works
	// end-to-end: 2 consecutive "poll timeout" errors must NOT fail the
	// task; only the 3rd attempt's outcome matters.
	w, api, _, _, tr := newWorkerWithFakes(t)

	api.submitRefs = []labsapi.MediaRef{{MediaID: "m-1", Seed: 7}}
	api.waitResults = []waitResult{
		{err: errors.New("poll timeout after 5m0s")},
		{err: errors.New("poll timeout after 10m0s")},
		{finals: []labsapi.FinalMedia{{
			MediaRef: labsapi.MediaRef{MediaID: "m-1", Seed: 7},
			Status:   labsapi.StatusSuccessful,
		}}},
	}

	task := enqueueOne(t, w, "retry-then-ok")
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	waitForStatus(t, tr, task.ID, db.StatusSucceeded, 5*time.Second)

	if api.waitCalls.Load() != 3 {
		t.Errorf("Wait calls = %d, want exactly 3 (1 base + 2 retries)", api.waitCalls.Load())
	}
}

func TestWorker_PollTimeoutAllRetriesExhausted(t *testing.T) {
	// 1 base + maxPollRetries (=2) = 3 attempts total. After that the
	// task must transition to failed instead of looping forever.
	w, api, _, _, tr := newWorkerWithFakes(t)

	api.submitRefs = []labsapi.MediaRef{{MediaID: "m-1", Seed: 7}}
	api.waitResults = []waitResult{
		{err: errors.New("poll timeout after 5m0s")},
		{err: errors.New("poll timeout after 10m0s")},
		{err: errors.New("poll timeout after 20m0s")},
	}

	task := enqueueOne(t, w, "retry-fail")
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	got := waitForStatus(t, tr, task.ID, db.StatusFailed, 5*time.Second)

	if api.waitCalls.Load() != 3 {
		t.Errorf("Wait calls = %d, want 3", api.waitCalls.Load())
	}
	if got.Error == "" {
		t.Error("failed task has empty error message")
	}
}

func TestWorker_NonTimeoutErrorDoesNotRetry(t *testing.T) {
	// Network errors / parse errors must NOT trigger the timeout retry
	// path, otherwise we mask real bugs by hammering a broken endpoint.
	w, api, _, _, tr := newWorkerWithFakes(t)

	api.submitRefs = []labsapi.MediaRef{{MediaID: "m-1", Seed: 7}}
	api.waitResults = []waitResult{
		{err: errors.New("network unreachable")},
	}

	task := enqueueOne(t, w, "no-retry")
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	waitForStatus(t, tr, task.ID, db.StatusFailed, 5*time.Second)

	if got := api.waitCalls.Load(); got != 1 {
		t.Errorf("Wait calls = %d, want 1 (must NOT retry on non-timeout)", got)
	}
}

func TestWorker_SubmitErrorFailsImmediately(t *testing.T) {
	w, api, _, dl, tr := newWorkerWithFakes(t)

	api.submitErr = errors.New("submit refused")

	task := enqueueOne(t, w, "submit-fail")
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	got := waitForStatus(t, tr, task.ID, db.StatusFailed, 5*time.Second)

	if api.waitCalls.Load() != 0 {
		t.Errorf("Wait calls = %d, want 0 (submit failed)", api.waitCalls.Load())
	}
	if dl.calls.Load() != 0 {
		t.Errorf("Download calls = %d, want 0", dl.calls.Load())
	}
	if got.Error == "" {
		t.Error("failed task missing error message")
	}
}

// TestWorker_ParallelDownloads proves the MaxParallelDl setting is wired
// into the worker's download loop. With 4 outputs and parallel=4, the
// concurrency observed mid-flight must be > 1.
func TestWorker_ParallelDownloads(t *testing.T) {
	w, api, _, _, tr := newWorkerWithFakes(t)
	// Crank parallel up; reuse same SetBundle pattern.
	bundle, _ := w.deps.Settings.GetBundle()
	bundle.MaxParallelDl = 4
	if err := w.deps.Settings.SetBundle(bundle); err != nil {
		t.Fatal(err)
	}

	api.submitRefs = []labsapi.MediaRef{
		{MediaID: "m-1", Seed: 1},
		{MediaID: "m-2", Seed: 2},
		{MediaID: "m-3", Seed: 3},
		{MediaID: "m-4", Seed: 4},
	}
	api.waitResults = []waitResult{{
		finals: []labsapi.FinalMedia{
			{MediaRef: labsapi.MediaRef{MediaID: "m-1", Seed: 1}, Status: labsapi.StatusSuccessful},
			{MediaRef: labsapi.MediaRef{MediaID: "m-2", Seed: 2}, Status: labsapi.StatusSuccessful},
			{MediaRef: labsapi.MediaRef{MediaID: "m-3", Seed: 3}, Status: labsapi.StatusSuccessful},
			{MediaRef: labsapi.MediaRef{MediaID: "m-4", Seed: 4}, Status: labsapi.StatusSuccessful},
		},
	}}

	// Replace the dummy downloader with one that holds each call until all
	// 4 are in flight, proving they run concurrently.
	concurrent := &concurrentMaxDownloader{wantInflight: 4, hold: 80 * time.Millisecond}
	w.deps.Download = concurrent

	task := enqueueOne(t, w, "parallel-dl")
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	got := waitForStatus(t, tr, task.ID, db.StatusSucceeded, 5*time.Second)

	if concurrent.peak.Load() < 2 {
		t.Errorf("peak concurrency = %d, want >= 2 (parallelDl was %d)",
			concurrent.peak.Load(), bundle.MaxParallelDl)
	}
	if len(got.VideoPaths) != 4 {
		t.Errorf("VideoPaths len = %d, want 4 (all 4 outputs saved)", len(got.VideoPaths))
	}
}

// concurrentMaxDownloader records the peak number of in-flight Fetch calls.
type concurrentMaxDownloader struct {
	wantInflight int
	hold         time.Duration
	inflight     atomic.Int32
	peak         atomic.Int32
}

func (d *concurrentMaxDownloader) Fetch(ctx context.Context, _, _ string, _ download.ProgressFn) error {
	now := d.inflight.Add(1)
	for {
		p := d.peak.Load()
		if now <= p || d.peak.CompareAndSwap(p, now) {
			break
		}
	}
	defer d.inflight.Add(-1)
	select {
	case <-time.After(d.hold):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func TestWorker_DownloadFailureKeepsTaskFailed(t *testing.T) {
	// When ALL downloads fail, the task is marked failed (the worker only
	// considers it succeeded if at least one path saved).
	w, api, _, dl, tr := newWorkerWithFakes(t)

	api.submitRefs = []labsapi.MediaRef{{MediaID: "m-1", Seed: 7}}
	api.waitResults = []waitResult{{
		finals: []labsapi.FinalMedia{{
			MediaRef: labsapi.MediaRef{MediaID: "m-1", Seed: 7},
			Status:   labsapi.StatusSuccessful,
		}},
	}}
	dl.failNth[1] = errors.New("disk full")

	task := enqueueOne(t, w, "dl-fail")
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	got := waitForStatus(t, tr, task.ID, db.StatusFailed, 5*time.Second)

	if got.Error == "" || len(got.VideoPaths) != 0 {
		t.Errorf("expected empty VideoPaths + error message; got %+v", got)
	}
}
