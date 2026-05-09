package labsapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"veo3-manager/internal/browser"
)

// fakePage implements PageFetcher with scriptable responses.
type fakePage struct {
	token            string
	tokenErr         error
	refreshCalls     atomic.Int32
	recaptcha        string
	recaptchaErr     error
	projectID        string
	ensureViewErr    error
	ensureViewCalls  atomic.Int32
	fetchCalls       atomic.Int32
	fetchByPath      map[string][]fetchResult // path -> sequence of results
	fetchPerPathIdx  map[string]int           // per-path call counter (mu-protected)
	fetchPathHistory []string
	mu               struct{}                 // unused; tests are single-goroutine
}

type fetchResult struct {
	body []byte
	err  error
}

func (f *fakePage) PageFetch(_ context.Context, _, fullURL, _, _ string) ([]byte, error) {
	f.fetchCalls.Add(1)
	// Strip query string for matching against path constants.
	path := fullURL
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimPrefix(path, BaseURL)
	f.fetchPathHistory = append(f.fetchPathHistory, path)

	results, ok := f.fetchByPath[path]
	if !ok || len(results) == 0 {
		return nil, fmt.Errorf("fakePage: no scripted result for %q", path)
	}

	if f.fetchPerPathIdx == nil {
		f.fetchPerPathIdx = make(map[string]int)
	}
	n := f.fetchPerPathIdx[path]
	if n >= len(results) {
		// Loop on the last entry once the script is exhausted (so a path
		// expected to be polled forever needn't list infinite entries).
		n = len(results) - 1
	} else {
		f.fetchPerPathIdx[path] = n + 1
	}
	r := results[n]
	return r.body, r.err
}

func (f *fakePage) Token(_ context.Context) (string, error) {
	return f.token, f.tokenErr
}

func (f *fakePage) Refresh(_ context.Context) error {
	f.refreshCalls.Add(1)
	f.tokenErr = nil // refresh "succeeded"
	return nil
}

func (f *fakePage) RecaptchaToken(_ context.Context) (string, error) {
	if f.recaptcha == "" && f.recaptchaErr == nil {
		return "fake-recaptcha-token", nil
	}
	return f.recaptcha, f.recaptchaErr
}

func (f *fakePage) ProjectID() string { return f.projectID }

func (f *fakePage) EnsureProjectView() error {
	f.ensureViewCalls.Add(1)
	return f.ensureViewErr
}

// jsonBody is a tiny helper so test data reads naturally.
func jsonBody(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSubmit_HappyPath(t *testing.T) {
	page := &fakePage{
		token:     "tok-abc",
		projectID: "proj-1",
		fetchByPath: map[string][]fetchResult{
			PathCredits[:strings.Index(PathCredits, "?")]: {{
				body: jsonBody(t, map[string]any{"credits": 100, "userPaygateTier": "PAYGATE_TIER_ONE"}),
			}},
			PathSubmit: {
				{body: jsonBody(t, map[string]any{"media": []map[string]any{{"name": "media-1", "projectId": "proj-1"}}})},
				{body: jsonBody(t, map[string]any{"media": []map[string]any{{"name": "media-2", "projectId": "proj-1"}}})},
			},
		},
	}

	c := NewClient(page, nil)
	refs, err := c.Submit(context.Background(), "a cat surfing", SubmitConfig{
		Model: DefaultModel, AspectRatio: "16:9", OutputCount: 2,
		Seeds: []int64{111, 222},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("len refs = %d, want 2", len(refs))
	}
	if refs[0].MediaID != "media-1" || refs[1].MediaID != "media-2" {
		t.Errorf("refs = %+v", refs)
	}
	if refs[0].Seed != 111 || refs[1].Seed != 222 {
		t.Errorf("seed not propagated: %+v", refs)
	}
	if page.ensureViewCalls.Load() < 1 {
		t.Errorf("EnsureProjectView called %d times, want >=1", page.ensureViewCalls.Load())
	}
}

func TestSubmit_RejectsEmptyPrompt(t *testing.T) {
	page := &fakePage{token: "x", projectID: "p"}
	c := NewClient(page, nil)
	_, err := c.Submit(context.Background(), "", SubmitConfig{OutputCount: 1})
	if err == nil {
		t.Fatal("Submit should reject empty prompt")
	}
}

func TestSubmit_NoProjectOpen(t *testing.T) {
	page := &fakePage{token: "x", projectID: ""}
	c := NewClient(page, nil)
	_, err := c.Submit(context.Background(), "hello", SubmitConfig{OutputCount: 1})
	if err == nil || !strings.Contains(err.Error(), "no Flow project") {
		t.Fatalf("err = %v, want 'no Flow project' message", err)
	}
}

func TestSubmit_RetriesOn401AndRefreshesToken(t *testing.T) {
	// First submit attempt returns 401-wrapped error; refresh then retry.
	// fetchJSON catches errors.Is(err, browser.ErrUnauthorized) and runs
	// page.Refresh() before doing one more attempt.
	page := &fakePage{
		token:     "stale-tok",
		projectID: "proj-1",
		fetchByPath: map[string][]fetchResult{
			PathCredits[:strings.Index(PathCredits, "?")]: {{
				body: jsonBody(t, map[string]any{"credits": 50, "userPaygateTier": "PAYGATE_TIER_ONE"}),
			}},
			PathSubmit: {
				{err: fmt.Errorf("%w: POST %s: token expired", browser.ErrUnauthorized, BaseURL+PathSubmit)},
				{body: jsonBody(t, map[string]any{"media": []map[string]any{{"name": "media-after-refresh", "projectId": "proj-1"}}})},
			},
		},
	}

	c := NewClient(page, nil)
	refs, err := c.Submit(context.Background(), "x", SubmitConfig{Model: DefaultModel, AspectRatio: "16:9", OutputCount: 1, Seeds: []int64{1}})
	if err != nil {
		t.Fatalf("Submit (after refresh): %v", err)
	}
	if len(refs) != 1 || refs[0].MediaID != "media-after-refresh" {
		t.Errorf("refs = %+v", refs)
	}
	if got := page.refreshCalls.Load(); got != 1 {
		t.Errorf("Refresh calls = %d, want exactly 1", got)
	}
}

func TestSubmit_NonAuthErrorDoesNotRefresh(t *testing.T) {
	// 5xx / parse errors must propagate immediately. No refresh, no retry.
	page := &fakePage{
		token:     "x",
		projectID: "proj-1",
		fetchByPath: map[string][]fetchResult{
			PathCredits[:strings.Index(PathCredits, "?")]: {{err: errors.New("internal server error")}},
		},
	}
	c := NewClient(page, nil)
	_, err := c.Submit(context.Background(), "x", SubmitConfig{Model: DefaultModel, AspectRatio: "16:9", OutputCount: 1})
	if err == nil {
		t.Fatal("expected error from credits call")
	}
	if page.refreshCalls.Load() != 0 {
		t.Errorf("Refresh called on non-401 error - that's wrong")
	}
}

func TestPollOnce_DecodesAllStatuses(t *testing.T) {
	page := &fakePage{
		token: "t",
		fetchByPath: map[string][]fetchResult{
			PathStatus: {{body: jsonBody(t, map[string]any{
				"media": []map[string]any{
					{
						"name":      "m-success",
						"projectId": "p",
						"mediaMetadata": map[string]any{
							"mediaStatus": map[string]any{"mediaGenerationStatus": StatusSuccessfulStr},
						},
						"video": map[string]any{
							"generatedVideo": map[string]any{"seed": 9001},
						},
					},
					{
						"name":      "m-pending",
						"projectId": "p",
						"mediaMetadata": map[string]any{
							"mediaStatus": map[string]any{"mediaGenerationStatus": StatusPendingStr},
						},
					},
					{
						"name":      "m-fail",
						"projectId": "p",
						"mediaMetadata": map[string]any{
							"mediaStatus": map[string]any{"mediaGenerationStatus": StatusFailedStr},
						},
					},
				},
			})}},
		},
	}
	c := NewClient(page, nil)
	refs := []MediaRef{
		{MediaID: "m-success", ProjectID: "p", Seed: 9001}, // seed should also fall back from server response
		{MediaID: "m-pending", ProjectID: "p", Seed: 0},
		{MediaID: "m-fail", ProjectID: "p", Seed: 555},
	}
	rows, err := c.PollOnce(context.Background(), refs)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0].Status != StatusSuccessful || rows[1].Status != StatusPending || rows[2].Status != StatusFailed {
		t.Errorf("statuses wrong: %+v", rows)
	}
	if !rows[0].Status.Terminal() {
		t.Error("Successful must be Terminal()")
	}
	if !rows[2].Status.Terminal() {
		t.Error("Failed must be Terminal()")
	}
	if rows[1].Status.Terminal() {
		t.Error("Pending must NOT be Terminal()")
	}
}

func TestPollOnce_EmptyRefs(t *testing.T) {
	page := &fakePage{}
	c := NewClient(page, nil)
	rows, err := c.PollOnce(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if rows != nil {
		t.Errorf("rows = %v, want nil for empty refs", rows)
	}
	// Crucially: must NOT make a network call.
	if page.fetchCalls.Load() != 0 {
		t.Errorf("fetchCalls = %d, want 0 for empty refs", page.fetchCalls.Load())
	}
}

func TestWait_ReturnsImmediatelyWhenAllTerminal(t *testing.T) {
	page := &fakePage{
		token: "t",
		fetchByPath: map[string][]fetchResult{
			PathStatus: {{body: jsonBody(t, map[string]any{
				"media": []map[string]any{
					{
						"name":      "m-1",
						"projectId": "p",
						"mediaMetadata": map[string]any{
							"mediaStatus": map[string]any{"mediaGenerationStatus": StatusSuccessfulStr},
						},
					},
				},
			})}},
		},
	}
	c := NewClient(page, nil)
	start := time.Now()
	rows, err := c.Wait(context.Background(),
		[]MediaRef{{MediaID: "m-1", ProjectID: "p"}},
		PollOptions{Interval: 1 * time.Second, Timeout: 5 * time.Second},
	)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Wait took %s, expected near-immediate return on first-poll-success", elapsed)
	}
}

func TestWait_RetriesUntilTerminalOrTimeout(t *testing.T) {
	// First two polls: pending. Third: successful.
	pendingResp := jsonBody(t, map[string]any{
		"media": []map[string]any{{
			"name":      "m-1",
			"projectId": "p",
			"mediaMetadata": map[string]any{
				"mediaStatus": map[string]any{"mediaGenerationStatus": StatusPendingStr},
			},
		}},
	})
	successResp := jsonBody(t, map[string]any{
		"media": []map[string]any{{
			"name":      "m-1",
			"projectId": "p",
			"mediaMetadata": map[string]any{
				"mediaStatus": map[string]any{"mediaGenerationStatus": StatusSuccessfulStr},
			},
		}},
	})

	page := &fakePage{
		token: "t",
		fetchByPath: map[string][]fetchResult{
			PathStatus: {
				{body: pendingResp},
				{body: pendingResp},
				{body: successResp},
			},
		},
	}
	c := NewClient(page, nil)
	rows, err := c.Wait(context.Background(),
		[]MediaRef{{MediaID: "m-1", ProjectID: "p"}},
		PollOptions{Interval: 5 * time.Millisecond, Timeout: 1 * time.Second},
	)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Status != StatusSuccessful {
		t.Errorf("status = %s", rows[0].Status)
	}
	if page.fetchCalls.Load() != 3 {
		t.Errorf("fetch calls = %d, want 3", page.fetchCalls.Load())
	}
}

func TestWait_TimeoutReturnsExplicitError(t *testing.T) {
	pendingResp := jsonBody(t, map[string]any{
		"media": []map[string]any{{
			"name":      "m-1",
			"projectId": "p",
			"mediaMetadata": map[string]any{
				"mediaStatus": map[string]any{"mediaGenerationStatus": StatusPendingStr},
			},
		}},
	})
	page := &fakePage{
		token: "t",
		fetchByPath: map[string][]fetchResult{
			PathStatus: {{body: pendingResp}},
		},
	}
	c := NewClient(page, nil)
	_, err := c.Wait(context.Background(),
		[]MediaRef{{MediaID: "m-1", ProjectID: "p"}},
		PollOptions{Interval: 5 * time.Millisecond, Timeout: 30 * time.Millisecond},
	)
	if err == nil {
		t.Fatal("Wait should time out")
	}
	if !strings.Contains(err.Error(), "poll timeout") {
		t.Errorf("err = %v, want 'poll timeout' substring (worker uses isPollTimeout to detect)", err)
	}
}

func TestWait_ContextCancelReturnsCtxErr(t *testing.T) {
	pendingResp := jsonBody(t, map[string]any{
		"media": []map[string]any{{
			"name":      "m-1",
			"projectId": "p",
			"mediaMetadata": map[string]any{
				"mediaStatus": map[string]any{"mediaGenerationStatus": StatusPendingStr},
			},
		}},
	})
	page := &fakePage{
		token: "t",
		fetchByPath: map[string][]fetchResult{
			PathStatus: {{body: pendingResp}},
		},
	}
	c := NewClient(page, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := c.Wait(ctx,
		[]MediaRef{{MediaID: "m-1", ProjectID: "p"}},
		PollOptions{Interval: 50 * time.Millisecond, Timeout: 5 * time.Second},
	)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestRandSeed_PositiveAndInRange(t *testing.T) {
	for i := 0; i < 200; i++ {
		s := RandSeed()
		if s <= 0 {
			t.Fatalf("seed must be positive, got %d", s)
		}
		if s > 0x7fffffff {
			t.Fatalf("seed must fit in int32 (Veo3 API rejects int64): %d", s)
		}
	}
}

func TestAspectRatioFor(t *testing.T) {
	tests := map[string]string{
		"16:9":    "VIDEO_ASPECT_RATIO_LANDSCAPE",
		"9:16":    "VIDEO_ASPECT_RATIO_PORTRAIT",
		"1:1":     "VIDEO_ASPECT_RATIO_SQUARE",
		"":        "VIDEO_ASPECT_RATIO_LANDSCAPE",
		"invalid": "VIDEO_ASPECT_RATIO_LANDSCAPE",
	}
	for in, want := range tests {
		if got := AspectRatioFor(in); got != want {
			t.Errorf("AspectRatioFor(%q) = %q, want %q", in, got, want)
		}
	}
}
