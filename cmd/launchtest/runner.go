package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-rod/rod"
)

// runOnce does the full cycle: probe → reuse|launch → connect → open labs.
// Verifies a Chrome window is alive afterward.
func runOnce(port int, profile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	chromePath, err := detectChrome("")
	if err != nil {
		return fmt.Errorf("detect chrome: %w", err)
	}
	log.Printf("[1] chrome: %s", chromePath)

	if err := os.MkdirAll(profile, 0o755); err != nil {
		return fmt.Errorf("mkdir profile: %w", err)
	}
	log.Printf("[2] profile: %s", profile)

	t0 := time.Now()
	wsURL, alive := probeReuse(port, 1, 700*time.Millisecond)
	if alive {
		log.Printf("[3] reuse OK in %s, ws=%s", time.Since(t0), wsURL)
	} else {
		log.Printf("[3] no chrome on :%d after %s — will launch fresh", port, time.Since(t0))
	}

	weOwn := false
	pid := 0
	if !alive {
		t1 := time.Now()
		if !portFree(port) {
			return fmt.Errorf("port %d busy but /json/version not responding — kill the process holding it", port)
		}
		ws, p, err := launchFresh(chromePath, profile, port)
		if err != nil {
			return fmt.Errorf("launchFresh: %w", err)
		}
		wsURL = ws
		pid = p
		weOwn = true
		log.Printf("[4] launched in %s, pid=%d, ws=%s", time.Since(t1), pid, ws)
	}

	t2 := time.Now()
	br := rod.New().ControlURL(wsURL)
	if err := br.Connect(); err != nil {
		return fmt.Errorf("rod connect: %w", err)
	}
	log.Printf("[5] rod connected in %s", time.Since(t2))

	t3 := time.Now()
	page, err := ensureLabsTab(ctx, br)
	if err != nil {
		return fmt.Errorf("ensureLabsTab: %w", err)
	}
	log.Printf("[6] labs tab ready in %s, url=%s", time.Since(t3), page.MustInfo().URL)

	procs := chromesUsingProfile(profile)
	log.Printf("[7] %d Chrome processes using profile (main + helpers)", procs)
	if procs == 0 {
		return fmt.Errorf("no Chrome process visible after launch (window may not have opened)")
	}

	res, err := page.Eval(`() => 1 + 1`)
	if err != nil {
		return fmt.Errorf("page eval: %w", err)
	}
	var two int
	_ = res.Value.Unmarshal(&two)
	if two != 2 {
		return fmt.Errorf("eval returned %d not 2", two)
	}
	log.Printf("[8] CDP eval OK")

	if weOwn {
		log.Printf("[9] we own this Chrome (we'd close on Stop())")
	} else {
		log.Printf("[9] we are reusing an existing Chrome")
	}
	return nil
}
