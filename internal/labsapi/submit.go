package labsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"veo3-manager/internal/db"

	"github.com/google/uuid"
)

// Submit triggers N video generations with the same batchId and different
// seeds. Each output is ONE POST to the API; the API returns a single
// media id per call. Returns the list of MediaRefs to feed into Wait().
func (c *Client) Submit(ctx context.Context, prompt string, cfg SubmitConfig) ([]MediaRef, error) {
	if prompt == "" {
		return nil, fmt.Errorf("empty prompt")
	}
	if cfg.OutputCount < 1 {
		cfg.OutputCount = 1
	}
	if cfg.OutputCount > 4 {
		cfg.OutputCount = 4
	}
	// Defensive normalization for callers that bypass the queue worker (which
	// also normalizes). Routing both through the same db helpers keeps a single
	// source of truth so the model family + Omni duration can't diverge between
	// the two paths.
	cfg.Model = db.NormalizeModelFamily(cfg.Model)
	cfg.OmniDurationSec = db.NormalizeOmniDuration(cfg.OmniDurationSec)
	if cfg.AspectRatio == "" {
		cfg.AspectRatio = "16:9"
	}

	// Always normalise the tab to a project root view BEFORE submit. Flow
	// auto-pushes /edit/<workflow> after each generation, and users can
	// navigate freely — without this, ProjectID() returns "" and submit
	// fails with "no Flow project open in Chrome".
	if err := c.page.EnsureProjectView(); err != nil {
		c.log.Warn("EnsureProjectView before submit failed (continuing)", "err", err)
	}
	projectID := c.page.ProjectID()
	if projectID == "" {
		return nil, fmt.Errorf("no Flow project open in Chrome — open a project at labs.google/fx/vi/tools/flow first")
	}

	tier, _, err := c.GetCredits(ctx)
	if err != nil {
		return nil, fmt.Errorf("credits: %w", err)
	}

	batchID := uuid.NewString()
	sessionID := fmt.Sprintf(";%d", time.Now().UnixMilli())
	aspectEnum := AspectRatioFor(cfg.AspectRatio)
	// Resolve the family id (cfg.Model) to the concrete videoModelKey the API
	// expects for this aspect ratio / Omni clip length.
	modelKey := VideoModelKeyFor(cfg.Model, cfg.AspectRatio, cfg.OmniDurationSec)

	seeds := cfg.Seeds
	if len(seeds) < cfg.OutputCount {
		seeds = make([]int64, cfg.OutputCount)
		for i := range seeds {
			seeds[i] = RandSeed()
		}
	}

	refs := make([]MediaRef, 0, cfg.OutputCount)
	for i := 0; i < cfg.OutputCount; i++ {
		recap, err := c.page.RecaptchaToken(ctx)
		if err != nil {
			return refs, fmt.Errorf("recaptcha (output %d): %w", i, err)
		}
		mid, err := c.submitOne(ctx, projectID, batchID, sessionID, tier, recap, prompt, aspectEnum, modelKey, seeds[i])
		if err != nil {
			return refs, fmt.Errorf("submit (output %d): %w", i, err)
		}
		refs = append(refs, MediaRef{
			MediaID:   mid,
			ProjectID: projectID,
			Seed:      seeds[i],
		})
	}
	return refs, nil
}

func (c *Client) submitOne(
	ctx context.Context,
	projectID, batchID, sessionID, tier, recap, prompt, aspectEnum, model string,
	seed int64,
) (string, error) {
	request := map[string]any{
		"aspectRatio": aspectEnum,
		"textInput": map[string]any{
			"structuredPrompt": map[string]any{
				"parts": []map[string]any{{"text": prompt}},
			},
		},
		"videoModelKey": model,
		"metadata":      map[string]any{},
		"seed":          seed,
	}
	body := map[string]any{
		"mediaGenerationContext": map[string]any{
			"batchId":                batchID,
			"audioFailurePreference": AudioFailurePreference,
		},
		"clientContext": map[string]any{
			"projectId":       projectID,
			"tool":            ToolName,
			"userPaygateTier": tier,
			"sessionId":       sessionID,
			"recaptchaContext": map[string]any{
				"token":           recap,
				"applicationType": ApplicationType,
			},
		},
		"requests":         []map[string]any{request},
		"useV2ModelConfig": true,
	}

	// Dump exact JSON about to POST at DEBUG — useful when verifying a newly
	// enabled model family resolves to the right videoModelKey for the chosen
	// aspect ratio (e.g. Quality portrait → veo_3_1_t2v_portrait). Kept at
	// DEBUG (not INFO) so production logs aren't bloated with full prompt text
	// on every submit.
	if reqDump, err := json.MarshalIndent(body, "", "  "); err == nil {
		c.log.Debug("labsapi.submit request",
			"projectId", projectID,
			"batchId", batchID,
			"seed", seed,
			"videoModelKey", model,
			"aspectRatio", aspectEnum,
			"body", string(reqDump),
		)
	}

	var raw json.RawMessage
	err := c.fetchJSON(ctx, "POST", PathSubmit, body, &raw)
	if err != nil {
		// fetchJSON's error string already contains "status %d: %s" for
		// non-2xx (see browser/page_fetch.go). Log verbatim so the exact
		// Labs reply (e.g. INVALID_ARGUMENT on an unknown model key) is
		// visible in the dev console + history task error field.
		c.log.Warn("labsapi.submit response error", "model", model, "err", err.Error())
		return "", err
	}
	c.log.Debug("labsapi.submit response ok", "model", model, "body", string(raw))

	var out struct {
		Media []struct {
			Name      string `json:"name"`
			ProjectID string `json:"projectId"`
		} `json:"media"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("decode submit: %w (body: %s)", err, snip(raw))
	}
	if len(out.Media) == 0 {
		return "", fmt.Errorf("submit returned no media")
	}
	return out.Media[0].Name, nil
}
