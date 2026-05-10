package labsapi

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrPollTimeout is returned by Wait when its deadline elapses before every
// media reaches a terminal status. The queue worker uses errors.Is on this
// to decide whether to extend the deadline and retry.
var ErrPollTimeout = errors.New("poll timeout")

// PollOnce sends a single batch status check.
func (c *Client) PollOnce(ctx context.Context, refs []MediaRef) ([]FinalMedia, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	media := make([]map[string]any, 0, len(refs))
	for _, r := range refs {
		media = append(media, map[string]any{"name": r.MediaID, "projectId": r.ProjectID})
	}
	body := map[string]any{"media": media}

	var out struct {
		Media []struct {
			Name          string `json:"name"`
			ProjectID     string `json:"projectId"`
			MediaMetadata struct {
				MediaStatus struct {
					MediaGenerationStatus string `json:"mediaGenerationStatus"`
				} `json:"mediaStatus"`
			} `json:"mediaMetadata"`
			Video struct {
				GeneratedVideo struct {
					Seed int64 `json:"seed"`
				} `json:"generatedVideo"`
			} `json:"video"`
		} `json:"media"`
	}
	if err := c.fetchJSON(ctx, "POST", PathStatus, body, &out); err != nil {
		return nil, err
	}

	rows := make([]FinalMedia, 0, len(out.Media))
	seedByID := map[string]int64{}
	for _, r := range refs {
		seedByID[r.MediaID] = r.Seed
	}
	for _, m := range out.Media {
		seed := m.Video.GeneratedVideo.Seed
		if seed == 0 {
			seed = seedByID[m.Name]
		}
		rows = append(rows, FinalMedia{
			MediaRef: MediaRef{MediaID: m.Name, ProjectID: m.ProjectID, Seed: seed},
			Status:   Status(m.MediaMetadata.MediaStatus.MediaGenerationStatus),
		})
	}
	return rows, nil
}

// Wait polls every Interval until every ref reaches a terminal status,
// the context is cancelled, or Timeout elapses.
func (c *Client) Wait(ctx context.Context, refs []MediaRef, opts PollOptions) ([]FinalMedia, error) {
	if opts.Interval <= 0 {
		opts.Interval = 10 * time.Second
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Minute
	}
	deadline := time.Now().Add(opts.Timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		rows, err := c.PollOnce(ctx, refs)
		if err != nil {
			return nil, err
		}
		allDone := true
		for _, r := range rows {
			if !r.Status.Terminal() {
				allDone = false
				break
			}
		}
		if allDone {
			return rows, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%w after %s", ErrPollTimeout, opts.Timeout)
		}

		t := time.NewTimer(opts.Interval)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
}
