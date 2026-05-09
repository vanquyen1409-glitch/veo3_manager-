// Standalone reliability test for the Chrome auto-launch flow used by the
// app's BrowserService. Runs the same `tryReuse → launchFresh → connect →
// open labs.google` sequence, prints every step + timing, and verifies
// that Chrome is actually visible after.
//
// Usage:
//
//	go run ./cmd/launchtest                  # 1 iteration with default profile
//	go run ./cmd/launchtest -n 3             # 3 iterations to test reliability
//	go run ./cmd/launchtest -profile <dir>   # custom user-data-dir
//	go run ./cmd/launchtest -kill-first      # kill any existing veo3 chrome first
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"
)

const labsURL = "https://labs.google/fx/vi/tools/flow"

func main() {
	port := flag.Int("port", 9222, "remote-debugging port")
	iterations := flag.Int("n", 1, "iterations")
	profileFlag := flag.String("profile", "", "user-data-dir (default %APPDATA%/veo3-manager/chromedata)")
	killFirst := flag.Bool("kill-first", false, "kill any existing chromes using the profile before each run")
	flag.Parse()

	profile := *profileFlag
	if profile == "" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
		profile = filepath.Join(appData, "veo3-manager", "chromedata")
	}

	fail := 0
	for i := 1; i <= *iterations; i++ {
		log.Printf("\n══════ iteration %d/%d ══════", i, *iterations)
		if *killFirst {
			killProfileChromes(profile)
			time.Sleep(800 * time.Millisecond)
		}
		if err := runOnce(*port, profile); err != nil {
			log.Printf("[iter %d] FAILED: %v", i, err)
			fail++
		} else {
			log.Printf("[iter %d] OK", i)
		}
	}
	log.Printf("\n══════ done: %d/%d ok ══════", *iterations-fail, *iterations)
	if fail > 0 {
		os.Exit(1)
	}
}
