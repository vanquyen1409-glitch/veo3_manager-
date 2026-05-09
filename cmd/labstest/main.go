// Standalone test program: drives the reverse-engineered Labs Flow API
// end-to-end (submit → poll → resolve URL) against the user's running
// Chrome session.
//
// IMPORTANT: All API calls happen INSIDE the browser tab via page.Eval +
// fetch(). Calling aisandbox-pa.googleapis.com from a Go http.Client trips
// reCAPTCHA Enterprise's origin / IP fingerprint check (`PUBLIC_ERROR_
// UNUSUAL_ACTIVITY`). Browser-context fetch carries the full Chrome session
// (cookies, origin, UA, network path) and Google trusts it.
//
//	cd cmd/labstest
//	go run . -ws "ws://127.0.0.1:9222/devtools/browser/<id>" \
//	         -prompt "A red sports car on neon city street"
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/go-rod/rod"
	"github.com/google/uuid"
)

const (
	apiBase = "https://aisandbox-pa.googleapis.com"
	pSubmit = "/v1/video:batchAsyncGenerateVideoText"
	pPoll   = "/v1/video:batchCheckAsyncVideoGenerationStatus"
	pCred   = "/v1/credits?key=AIzaSyBtrm0o5ab1c-Ec8ZuLcGt3oJAA5VWt3pY"

	model    = "veo_3_1_t2v_fast"
	aspect   = "VIDEO_ASPECT_RATIO_LANDSCAPE"
	toolName = "PINHOLE"

	recaptchaSiteKey = "6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV"
	recaptchaAction  = "VIDEO_GENERATION"
)

func main() {
	wsURL := flag.String("ws", "", "Chrome CDP WebSocket URL (required)")
	prompt := flag.String("prompt", "A red sports car driving through neon city at night", "video prompt")
	count := flag.Int("count", 1, "number of outputs (1..4)")
	flag.Parse()
	if *wsURL == "" {
		log.Fatal("-ws is required")
	}
	if *count < 1 || *count > 4 {
		log.Fatalf("-count must be 1..4, got %d", *count)
	}

	browser := rod.New().ControlURL(*wsURL)
	if err := browser.Connect(); err != nil {
		log.Fatalf("connect chrome: %v", err)
	}

	page, projectID, err := findLabsTab(browser)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[ctx] page=%s project=%s", page.MustInfo().URL, projectID)

	token, err := readToken(page)
	if err != nil {
		log.Fatalf("token: %v", err)
	}
	log.Printf("[ctx] token=%s…(%d)", token[:24], len(token))

	tier, credits, err := pageGetCredits(page, token)
	if err != nil {
		log.Fatalf("credits: %v", err)
	}
	log.Printf("[ctx] credits=%d tier=%s", credits, tier)

	mediaIDs, err := submitBatch(page, token, projectID, *prompt, *count)
	if err != nil {
		log.Fatal(err)
	}

	finals, err := pageWait(page, token, projectID, mediaIDs, 5*time.Minute)
	if err != nil {
		log.Fatalf("poll: %v", err)
	}
	for _, f := range finals {
		log.Printf("[final] media=%s status=%s", f.Name, f.Status)
	}

	resolveAll(browser, finals)
	log.Println("[done]")
}

func submitBatch(page *rod.Page, token, projectID, prompt string, count int) ([]string, error) {
	batchID := uuid.NewString()
	sessionID := fmt.Sprintf(";%d", time.Now().UnixMilli())
	tier, _, err := pageGetCredits(page, token)
	if err != nil {
		return nil, fmt.Errorf("credits: %w", err)
	}
	mediaIDs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		recap, err := getRecaptchaToken(page)
		if err != nil {
			return nil, fmt.Errorf("recaptcha #%d: %w", i, err)
		}
		log.Printf("[submit %d/%d] recap len=%d", i+1, count, len(recap))
		mid, err := pageSubmit(page, token, projectID, batchID, sessionID, tier, recap, prompt, randSeed())
		if err != nil {
			return nil, fmt.Errorf("submit #%d: %w", i, err)
		}
		log.Printf("[submit %d/%d] media=%s", i+1, count, mid)
		mediaIDs = append(mediaIDs, mid)
	}
	return mediaIDs, nil
}

func resolveAll(b *rod.Browser, finals []finalRow) {
	for _, f := range finals {
		if f.Status != "MEDIA_GENERATION_STATUS_SUCCESSFUL" {
			log.Printf("[skip] %s status=%s", f.Name, f.Status)
			continue
		}
		gcs, err := resolveGCS(b, f.Name)
		if err != nil {
			log.Printf("[resolve %s] err=%v", f.Name, err)
			continue
		}
		log.Printf("[resolve %s] gcs=%s", f.Name, gcs)
	}
}
