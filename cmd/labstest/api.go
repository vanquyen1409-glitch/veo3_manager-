package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-rod/rod"
)

type finalRow struct {
	Name   string
	Status string
}

func pageGetCredits(p *rod.Page, token string) (string, int, error) {
	raw, err := pageFetch(p, "GET", apiBase+pCred, token, "")
	if err != nil {
		return "", 0, err
	}
	var out struct {
		Credits         int    `json:"credits"`
		UserPaygateTier string `json:"userPaygateTier"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, err
	}
	if out.UserPaygateTier == "" {
		out.UserPaygateTier = "PAYGATE_TIER_ONE"
	}
	return out.UserPaygateTier, out.Credits, nil
}

func pageSubmit(p *rod.Page, token, projectID, batchID, sessionID, tier, recap, prompt string, seed int64) (string, error) {
	body := map[string]any{
		"mediaGenerationContext": map[string]any{
			"batchId":                batchID,
			"audioFailurePreference": "BLOCK_SILENCED_VIDEOS",
		},
		"clientContext": map[string]any{
			"projectId":       projectID,
			"tool":            toolName,
			"userPaygateTier": tier,
			"sessionId":       sessionID,
			"recaptchaContext": map[string]any{
				"token":           recap,
				"applicationType": "RECAPTCHA_APPLICATION_TYPE_WEB",
			},
		},
		"requests": []map[string]any{{
			"aspectRatio": aspect,
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
	bodyJSON, _ := json.Marshal(body)
	raw, err := pageFetch(p, "POST", apiBase+pSubmit, token, string(bodyJSON))
	if err != nil {
		return "", err
	}
	var out struct {
		Media []struct {
			Name string `json:"name"`
		} `json:"media"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("decode submit: %w (body: %s)", err, snip(string(raw)))
	}
	if len(out.Media) == 0 {
		return "", fmt.Errorf("submit returned no media")
	}
	return out.Media[0].Name, nil
}

func pageWait(p *rod.Page, token, projectID string, mediaIDs []string, timeout time.Duration) ([]finalRow, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body := map[string]any{
			"media": func() []map[string]any {
				out := make([]map[string]any, 0, len(mediaIDs))
				for _, m := range mediaIDs {
					out = append(out, map[string]any{"name": m, "projectId": projectID})
				}
				return out
			}(),
		}
		bodyJSON, _ := json.Marshal(body)
		raw, err := pageFetch(p, "POST", apiBase+pPoll, token, string(bodyJSON))
		if err != nil {
			return nil, err
		}
		var out struct {
			Media []struct {
				Name          string `json:"name"`
				MediaMetadata struct {
					MediaStatus struct {
						MediaGenerationStatus string `json:"mediaGenerationStatus"`
					} `json:"mediaStatus"`
				} `json:"mediaMetadata"`
			} `json:"media"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("decode poll: %w", err)
		}
		allDone := true
		rows := make([]finalRow, 0, len(out.Media))
		for _, m := range out.Media {
			s := m.MediaMetadata.MediaStatus.MediaGenerationStatus
			rows = append(rows, finalRow{Name: m.Name, Status: s})
			if s != "MEDIA_GENERATION_STATUS_SUCCESSFUL" && s != "MEDIA_GENERATION_STATUS_FAILED" {
				allDone = false
			}
		}
		log.Printf("[poll] %v", rows)
		if allDone {
			return rows, nil
		}
		time.Sleep(10 * time.Second)
	}
	return nil, fmt.Errorf("poll timeout")
}
