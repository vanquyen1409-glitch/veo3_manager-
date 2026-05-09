package labsapi

import (
	"context"
	"fmt"
	"time"

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
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
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
		mid, err := c.submitOne(ctx, projectID, batchID, sessionID, tier, recap, prompt, aspectEnum, cfg.Model, seeds[i])
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
		"requests": []map[string]any{{
			"aspectRatio": aspectEnum,
			"textInput": map[string]any{
				"structuredPrompt": map[string]any{
					"parts": []map[string]any{{"text": prompt}},
				},
			},
			"videoModelKey": model,
			"metadata":      map[string]any{},
			"seed":          seed,
		}},
		"useV2ModelConfig": true,
	}

	var out struct {
		Media []struct {
			Name      string `json:"name"`
			ProjectID string `json:"projectId"`
		} `json:"media"`
	}
	if err := c.fetchJSON(ctx, "POST", PathSubmit, body, &out); err != nil {
		return "", err
	}
	if len(out.Media) == 0 {
		return "", fmt.Errorf("submit returned no media")
	}
	return out.Media[0].Name, nil
}
