package db

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// newTestDB opens a fresh SQLite DB inside a temp dir, runs migrations, and
// returns the open *sql.DB plus a TaskRepo wired to it. Cleanup is automatic
// via t.TempDir.
func newTestDB(t *testing.T) (*TaskRepo, *SettingsRepo) {
	t.Helper()
	dir := t.TempDir()
	sqlDB, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return NewTaskRepo(sqlDB), NewSettingsRepo(sqlDB, DefaultDirs{
		UserDataDir: filepath.Join(dir, "chromedata"),
		OutputDir:   filepath.Join(dir, "videos"),
	})
}

func TestMigrate_FreshDB(t *testing.T) {
	tr, _ := newTestDB(t)
	// If migrations succeeded the tasks table is queryable.
	if _, err := tr.List(ListFilter{}); err != nil {
		t.Fatalf("List on fresh DB: %v", err)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	// Second Open on the same dir must be a no-op (current schema_version
	// >= every migration). Catches the bug where someone forgets to gate
	// migration N on `m.version > current`.
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		sqlDB, err := Open(dir)
		if err != nil {
			t.Fatalf("Open #%d: %v", i+1, err)
		}
		_ = sqlDB.Close()
	}
}

func mustEnqueue(t *testing.T, tr *TaskRepo, prompt string) Task {
	t.Helper()
	task := Task{
		ID:        uuid.NewString(),
		Prompt:    prompt,
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
		Config: GenerationConfig{
			Model: "veo_3_1_t2v_fast", AspectRatio: "16:9", OutputCount: 1,
		},
	}
	cfgJSON, _ := json.Marshal(task.Config)
	if _, err := tr.db.Exec(
		`INSERT INTO tasks (id, prompt, config_json, status, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		task.ID, task.Prompt, string(cfgJSON), string(task.Status), task.CreatedAt,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return task
}

func TestNextPending_FlipsToRunning(t *testing.T) {
	tr, _ := newTestDB(t)
	want := mustEnqueue(t, tr, "first")

	got, err := tr.NextPending()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("NextPending returned nil")
	}
	if got.ID != want.ID {
		t.Errorf("got id %s, want %s", got.ID, want.ID)
	}
	if got.Status != StatusRunning {
		t.Errorf("status = %s, want running", got.Status)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", got.Attempts)
	}
}

func TestNextPending_ReturnsNilWhenEmpty(t *testing.T) {
	tr, _ := newTestDB(t)
	got, err := tr.NextPending()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}
}

func TestNextPending_FIFOOrder(t *testing.T) {
	tr, _ := newTestDB(t)
	// Insert three with explicit timestamps so we can assert FIFO.
	a := mustEnqueue(t, tr, "a")
	time.Sleep(20 * time.Millisecond)
	b := mustEnqueue(t, tr, "b")
	time.Sleep(20 * time.Millisecond)
	_ = mustEnqueue(t, tr, "c")

	first, _ := tr.NextPending()
	if first.ID != a.ID {
		t.Errorf("first claimed = %s, want %s (oldest)", first.ID, a.ID)
	}
	second, _ := tr.NextPending()
	if second.ID != b.ID {
		t.Errorf("second claimed = %s, want %s", second.ID, b.ID)
	}
}

func TestUpdateStatus_SetsFinishedAtOnTerminal(t *testing.T) {
	tr, _ := newTestDB(t)
	task := mustEnqueue(t, tr, "x")
	_, _ = tr.NextPending() // -> running
	if err := tr.UpdateStatus(task.ID, StatusSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	got, err := tr.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusSucceeded {
		t.Errorf("status = %s", got.Status)
	}
	if got.FinishedAt == nil {
		t.Error("FinishedAt is nil after terminal transition")
	}
}

func TestUpdateStatus_NoFinishedAtOnNonTerminal(t *testing.T) {
	tr, _ := newTestDB(t)
	task := mustEnqueue(t, tr, "x")
	if err := tr.UpdateStatus(task.ID, StatusRunning, ""); err != nil {
		t.Fatal(err)
	}
	got, _ := tr.Get(task.ID)
	if got.FinishedAt != nil {
		t.Error("FinishedAt should remain nil on non-terminal")
	}
}

func TestUpdateStatus_PreservesErrorOnEmptyMsg(t *testing.T) {
	// Pass empty errMsg should NOT clobber an existing error column.
	tr, _ := newTestDB(t)
	task := mustEnqueue(t, tr, "x")
	_ = tr.UpdateStatus(task.ID, StatusFailed, "boom")
	_ = tr.UpdateStatus(task.ID, StatusFailed, "") // emptyMsg
	got, _ := tr.Get(task.ID)
	if got.Error != "boom" {
		t.Errorf("Error = %q, want preserved 'boom'", got.Error)
	}
}

func TestAppendOutput_AtomicAndAdditive(t *testing.T) {
	// This is the json_insert fix from Tech Debt #2 - guarantees no read-
	// modify-write race and survives a crash mid-append.
	tr, _ := newTestDB(t)
	task := mustEnqueue(t, tr, "x")

	if err := tr.AppendOutput(task.ID, "/out/1.mp4"); err != nil {
		t.Fatal(err)
	}
	if err := tr.AppendOutput(task.ID, "/out/2.mp4"); err != nil {
		t.Fatal(err)
	}
	if err := tr.AppendOutput(task.ID, "/out/3.mp4"); err != nil {
		t.Fatal(err)
	}

	got, err := tr.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/out/1.mp4", "/out/2.mp4", "/out/3.mp4"}
	if len(got.VideoPaths) != len(want) {
		t.Fatalf("len(VideoPaths) = %d, want %d (got %v)", len(got.VideoPaths), len(want), got.VideoPaths)
	}
	for i, p := range want {
		if got.VideoPaths[i] != p {
			t.Errorf("VideoPaths[%d] = %q, want %q", i, got.VideoPaths[i], p)
		}
	}
}

func TestAppendOutput_HandlesNullColumn(t *testing.T) {
	// Older rows might have video_paths IS NULL. json_insert with COALESCE
	// must still produce a valid array.
	tr, _ := newTestDB(t)
	task := mustEnqueue(t, tr, "x")
	if err := tr.AppendOutput(task.ID, "/only/one.mp4"); err != nil {
		t.Fatal(err)
	}
	got, _ := tr.Get(task.ID)
	if len(got.VideoPaths) != 1 || got.VideoPaths[0] != "/only/one.mp4" {
		t.Errorf("VideoPaths = %v", got.VideoPaths)
	}
}

func TestResetStaleRunning_FlipsRunningToPending(t *testing.T) {
	tr, _ := newTestDB(t)
	a := mustEnqueue(t, tr, "running-1")
	_, _ = tr.NextPending() // a -> running
	b := mustEnqueue(t, tr, "running-2")
	_, _ = tr.NextPending() // b -> running
	c := mustEnqueue(t, tr, "still-pending")

	n, err := tr.ResetStaleRunning()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("affected = %d, want 2", n)
	}
	gotA, _ := tr.Get(a.ID)
	gotB, _ := tr.Get(b.ID)
	gotC, _ := tr.Get(c.ID)
	if gotA.Status != StatusPending {
		t.Errorf("a status = %s, want pending", gotA.Status)
	}
	if gotB.Status != StatusPending {
		t.Errorf("b status = %s, want pending", gotB.Status)
	}
	if gotC.Status != StatusPending {
		t.Errorf("c status = %s, want still pending", gotC.Status)
	}
}

func TestList_StatusFilter(t *testing.T) {
	tr, _ := newTestDB(t)
	a := mustEnqueue(t, tr, "a")
	b := mustEnqueue(t, tr, "b")
	c := mustEnqueue(t, tr, "c")
	_ = tr.UpdateStatus(a.ID, StatusSucceeded, "")
	_ = tr.UpdateStatus(b.ID, StatusFailed, "x")
	// c stays pending

	got, err := tr.List(ListFilter{Statuses: []TaskStatus{StatusFailed, StatusSucceeded}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, row := range got {
		if row.Status != StatusFailed && row.Status != StatusSucceeded {
			t.Errorf("unexpected status %s", row.Status)
		}
		if row.ID == c.ID {
			t.Error("pending task leaked into filtered list")
		}
	}
}

func TestList_OrderByAllowlist_BlocksUnknown(t *testing.T) {
	// Tech Debt #3 fix: any value outside the allowlist must fall back to
	// created_at and produce a normal result, never an error or injection.
	tr, _ := newTestDB(t)
	mustEnqueue(t, tr, "a")
	mustEnqueue(t, tr, "b")

	rows, err := tr.List(ListFilter{
		// A common SQL injection probe: `id; DROP TABLE tasks;--`
		OrderBy:  "id; DROP TABLE tasks;--",
		OrderDir: "DESC; SELECT * FROM settings;",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("List with hostile inputs returned error %v - allowlist should normalise silently", err)
	}
	if len(rows) != 2 {
		t.Errorf("got %d rows, want 2 (table not dropped)", len(rows))
	}
	// Verify schema still intact.
	if _, err := tr.List(ListFilter{}); err != nil {
		t.Fatalf("schema corrupted after hostile orderBy? %v", err)
	}
}

func TestList_LimitClampsTo500AndDefaultsTo50(t *testing.T) {
	tr, _ := newTestDB(t)
	for i := 0; i < 60; i++ {
		mustEnqueue(t, tr, "t")
	}
	rows, _ := tr.List(ListFilter{}) // default limit 50
	if len(rows) != 50 {
		t.Errorf("default limit got %d, want 50", len(rows))
	}
	rows, _ = tr.List(ListFilter{Limit: 999}) // clamp 500
	if len(rows) > 60 {
		t.Errorf("clamp failed - got %d > 60", len(rows))
	}
}

func TestSettingsBundle_RoundTrip(t *testing.T) {
	_, sr := newTestDB(t)
	want := SettingsBundle{
		ChromePath:      "C:/chrome",
		UserDataDir:     "C:/data",
		OutputDir:       "C:/out",
		ChromeDebugPort: 9222,
		DefaultConfig: GenerationConfig{
			Model: "veo_3_1_t2v_fast", AspectRatio: "9:16", OutputCount: 2, SeedBase: 100, OutputDir: "C:/out",
		},
		PollIntervalMs: 5000,
		PollTimeoutMs:  600000,
		MaxParallelDl:  2,
	}
	if err := sr.SetBundle(want); err != nil {
		t.Fatal(err)
	}
	got, err := sr.GetBundle()
	if err != nil {
		t.Fatal(err)
	}
	// Path normalisation may flip '/' to '\' on Windows; compare via Clean.
	if filepath.Clean(got.ChromePath) != filepath.Clean(want.ChromePath) {
		t.Errorf("ChromePath = %q, want %q", got.ChromePath, want.ChromePath)
	}
	if got.ChromeDebugPort != want.ChromeDebugPort {
		t.Errorf("ChromeDebugPort = %d", got.ChromeDebugPort)
	}
	if got.DefaultConfig.Model != want.DefaultConfig.Model {
		t.Errorf("DefaultConfig.Model = %q", got.DefaultConfig.Model)
	}
	if got.DefaultConfig.AspectRatio != want.DefaultConfig.AspectRatio {
		t.Errorf("DefaultConfig.AspectRatio = %q", got.DefaultConfig.AspectRatio)
	}
}

func TestSettingsCacheInvalidates(t *testing.T) {
	// SetBundle must invalidate the GetBundle cache so the next read returns
	// fresh values. Catches the regression where cache leaks a stale port
	// after the user changes it in settings UI.
	_, sr := newTestDB(t)
	_ = sr.SetBundle(SettingsBundle{
		ChromeDebugPort: 9222, PollIntervalMs: 1000, PollTimeoutMs: 60000, MaxParallelDl: 1,
		UserDataDir: "/d", OutputDir: "/o",
		DefaultConfig: GenerationConfig{Model: "m", AspectRatio: "16:9", OutputCount: 1, OutputDir: "/o"},
	})
	if _, err := sr.GetBundle(); err != nil {
		t.Fatal(err)
	}
	_ = sr.SetBundle(SettingsBundle{
		ChromeDebugPort: 9333, PollIntervalMs: 2000, PollTimeoutMs: 120000, MaxParallelDl: 2,
		UserDataDir: "/d", OutputDir: "/o",
		DefaultConfig: GenerationConfig{Model: "m", AspectRatio: "16:9", OutputCount: 1, OutputDir: "/o"},
	})
	got, _ := sr.GetBundle()
	if got.ChromeDebugPort != 9333 {
		t.Errorf("cache leaked stale port: %d", got.ChromeDebugPort)
	}
}

func TestRequeue_CreatesNewPendingClone(t *testing.T) {
	tr, _ := newTestDB(t)
	src := mustEnqueue(t, tr, "original")
	_ = tr.UpdateStatus(src.ID, StatusFailed, "boom")

	clone, err := tr.Requeue(src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if clone.ID == src.ID {
		t.Error("clone has same ID as source")
	}
	if clone.Status != StatusPending {
		t.Errorf("clone status = %s, want pending", clone.Status)
	}
	if clone.SourceTaskID != src.ID {
		t.Errorf("SourceTaskID = %q, want %q", clone.SourceTaskID, src.ID)
	}
	if !strings.EqualFold(clone.Prompt, src.Prompt) {
		t.Errorf("Prompt drifted: %q vs %q", clone.Prompt, src.Prompt)
	}
}

func TestDelete_RemovesRow(t *testing.T) {
	tr, _ := newTestDB(t)
	task := mustEnqueue(t, tr, "doomed")
	if err := tr.Delete(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Get(task.ID); err == nil {
		t.Error("Get returned no error after Delete")
	}
}
