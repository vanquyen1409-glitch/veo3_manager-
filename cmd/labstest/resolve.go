package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func resolveGCS(b *rod.Browser, mediaName string) (string, error) {
	redirectURL := fmt.Sprintf(
		"https://labs.google/fx/api/trpc/media.getMediaUrlRedirect?name=%s",
		mediaName,
	)
	page, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return "", err
	}
	defer page.Close()

	enable := proto.NetworkEnable{}
	if err := enable.Call(page); err != nil {
		return "", err
	}

	// Capture every response so we can find the GCS hop even if rod doesn't
	// emit NetworkResponseReceived for the cross-origin redirect itself.
	urls := []string{}
	stop := page.EachEvent(func(e *proto.NetworkResponseReceived) {
		urls = append(urls, e.Response.URL)
	})
	go stop()

	if err := page.Navigate(redirectURL); err != nil {
		return "", err
	}
	_ = page.WaitLoad()
	time.Sleep(2 * time.Second)

	if u, ok := matchedGCS(currentPageURL(page)); ok {
		log.Printf("[resolve] page final URL: %s", u)
		return u, nil
	}

	for _, u := range urls {
		if matched, ok := matchedGCS(u); ok {
			return matched, nil
		}
	}

	if u, ok := matchedGCS(extractDOMURL(page)); ok {
		log.Printf("[resolve] DOM-extracted URL: %s", u)
		return u, nil
	}

	log.Printf("[resolve] captured %d responses, last 5: %v", len(urls), tail(urls, 5))
	return "", fmt.Errorf("gcs URL not captured")
}

func currentPageURL(page *rod.Page) string {
	info, _ := page.Info()
	if info == nil {
		return ""
	}
	return info.URL
}

func extractDOMURL(page *rod.Page) string {
	res, err := page.Eval(`() => {
		const v = document.querySelector('video');
		if (v && v.src) return v.src;
		const a = document.querySelector('a[href*="flow-content.google"], a[href*="googleusercontent.com"], a[href*="storage.googleapis.com"]');
		if (a) return a.href;
		return location.href;
	}`)
	if err != nil {
		return ""
	}
	var u string
	_ = res.Value.Unmarshal(&u)
	return u
}

func matchedGCS(u string) (string, bool) {
	if u == "" {
		return "", false
	}
	if strings.Contains(u, "flow-content.google") ||
		strings.Contains(u, "googleusercontent.com") ||
		strings.Contains(u, "storage.googleapis.com") {
		return u, true
	}
	return "", false
}

func tail[T any](s []T, n int) []T {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func randSeed() int64 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	v := int64(binary.LittleEndian.Uint32(b[:]) & 0x7fffffff)
	if v == 0 {
		v = 1
	}
	return v
}
