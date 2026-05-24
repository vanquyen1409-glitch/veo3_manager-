// Standalone probe: connects to the user's already-running Chrome (the one
// VEO3 Manager launched with --remote-debugging-port), finds the labs.google
// Flow tab, and digs the REAL videoModelKey strings out of the page's loaded
// JavaScript bundles. No video is generated, so it costs ZERO Flow credits.
//
// Why this exists: the Flow web UI shows model NAMES (Omni Flash, Veo 3.1
// Lite/Fast/Quality) but the API wants internal KEYS (e.g. veo_3_1_t2v_fast).
// Only Fast's key was captured from a real submit; the other three are still
// guesses (the "<TBD:...>" placeholders in frontend ConfigBar.tsx). This probe
// reads them straight from Google's frontend code so we can fill them in
// without spending credits on a real generation.
//
// Usage (Chrome must be open on the Flow project page, launched by the app):
//
//	go run ./cmd/probemodels            # auto-discovers port 9222
//	go run ./cmd/probemodels -port 9223 # custom debug port
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

func main() {
	port := flag.Int("port", 9222, "Chrome remote-debugging port the app launched")
	jsFile := flag.String("js", "", "path to a custom JS file to eval instead of the built-in probe")
	flag.Parse()

	wsURL, err := browserWSURL(*port)
	if err != nil {
		log.Fatalf("find Chrome debug endpoint on port %d: %v\n"+
			"→ Make sure VEO3 Manager is running and Chrome is connected (green dot).", *port, err)
	}

	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		log.Fatalf("connect chrome: %v", err)
	}

	page, err := findLabsTab(browser)
	if err != nil {
		log.Fatalf("%v\n→ Open the Flow project page in the app's Chrome window first.", err)
	}
	log.Printf("[ok] found Labs tab: %s", page.MustInfo().URL)

	js := probeJS
	if *jsFile != "" {
		b, err := os.ReadFile(*jsFile)
		if err != nil {
			log.Fatalf("read -js file: %v", err)
		}
		js = string(b)
	}

	out, err := runProbe(page, js)
	if err != nil {
		log.Fatalf("probe eval: %v", err)
	}

	fmt.Println("\n================ MODEL KEY PROBE RESULT ================")
	fmt.Println("Copy EVERYTHING below this line and send it back.")
	fmt.Println()
	fmt.Println(out)
	fmt.Println("========================================================")
}

// browserWSURL hits the CDP /json/version endpoint and returns the browser-
// level WebSocket debugger URL that rod connects to.
func browserWSURL(port int) (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return "", fmt.Errorf("decode /json/version: %w (body: %s)", err, string(b))
	}
	if v.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("no webSocketDebuggerUrl in /json/version")
	}
	return v.WebSocketDebuggerURL, nil
}

func findLabsTab(b *rod.Browser) (*rod.Page, error) {
	pages, err := b.Pages()
	if err != nil {
		return nil, err
	}
	for _, p := range pages {
		info, _ := p.Info()
		if info != nil && strings.Contains(info.URL, "labs.google") {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no labs.google tab open")
}

// probeJS searches the page's RUNTIME data (Next.js streaming payload
// self.__next_f, __NEXT_DATA__, and all inline scripts) — that's where the
// per-user model list lives, since the dropdown is populated at runtime, not
// hardcoded in the static JS bundles. For each model display label it prints
// the surrounding serialized text (the real videoModelKey + credit cost sit
// right next to the label in the model object). As a backstop it also walks
// the live DOM/React fibers for any node whose text is a model label and dumps
// the nearest fiber props.
const probeJS = `
async () => {
  const labels = ['Omni Flash', 'Veo 3.1 - Lite', 'Veo 3.1 - Fast', 'Veo 3.1 - Quality'];

  // 1) Gather every runtime data string the page holds.
  const blobs = [];
  try {
    const nf = self.__next_f;
    if (Array.isArray(nf)) nf.forEach(e => { if (Array.isArray(e)) e.forEach(x => { if (typeof x === 'string') blobs.push(x); }); });
  } catch (e) {}
  document.querySelectorAll('script:not([src])').forEach(s => { if (s.textContent) blobs.push(s.textContent); });
  const big = blobs.join('\n');

  // 2) Print context around each model label.
  const ctx = [];
  for (const lab of labels) {
    let i = big.indexOf(lab);
    while (i !== -1) {
      ctx.push(lab + '  =>  ...' + big.slice(Math.max(0, i - 300), i + 120).replace(/\s+/g, ' ') + '...');
      i = big.indexOf(lab, i + 1);
    }
  }

  // 3) Collect candidate keys + any "...ModelKey":"..." / "model":"..." pairs.
  const keys = new Set();
  (big.match(/veo[_a-z0-9]{3,}/gi) || []).forEach(x => keys.add(x));
  (big.match(/omni[_a-z0-9]{2,}/gi) || []).forEach(x => keys.add(x));
  const pairs = new Set();
  // The i18n table maps "video_model_<KEY>" -> display label. This is the
  // Rosetta stone for label↔key, e.g. video_model_veo_3_1_fast => "Veo 3.1 - Fast".
  (big.match(/"video_model_[^"]+":\{"message":"[^"]*"\}/g) || []).forEach(x => pairs.add(x));
  (big.match(/"videoModelKey"\s*:\s*"[^"]+"/g) || []).forEach(x => pairs.add(x));

  // 4) React fiber heap walk: find the model OPTIONS array. We DFS the whole
  // fiber tree and snapshot any memoizedProps/memoizedState object whose JSON
  // mentions >=2 model labels (that's the options list, which carries the real
  // videoModelKey next to each label). Works whether or not the dropdown is open.
  const dom = [];
  const seen = new Set();
  function objHasLabels(s) { let n = 0; for (const l of labels) if (s.includes(l)) n++; return n >= 2; }
  function snap(tag, val) {
    let s;
    try { s = JSON.stringify(val, (k, v) => typeof v === 'function' ? '[fn]' : v); } catch (e) { return; }
    if (!s || s.length < 20 || !objHasLabels(s)) return;
    const head = s.slice(0, 120);
    if (seen.has(head)) return;
    seen.add(head);
    dom.push(tag + '  =>  ' + s.slice(0, 2500));
  }
  // find a fiber root
  let root = null;
  for (const el of document.querySelectorAll('body *')) {
    const ck = Object.keys(el).find(k => k.startsWith('__reactContainer$'));
    if (ck) { root = el[ck]; break; }
    const fk = Object.keys(el).find(k => k.startsWith('__reactFiber$'));
    if (fk && !root) root = el[fk];
  }
  let count = 0;
  const stack = root ? [root.current || root] : [];
  while (stack.length && count < 200000) {
    const f = stack.pop(); count++;
    if (!f || typeof f !== 'object') continue;
    if (f.memoizedProps) snap('props', f.memoizedProps);
    if (f.memoizedState) snap('state', f.memoizedState);
    if (f.child) stack.push(f.child);
    if (f.sibling) stack.push(f.sibling);
  }

  return '=== blobs:' + blobs.length + ' totalLen:' + big.length + ' ===\n\n' +
    '=== CONTEXT (label -> nearby runtime data) ===\n' + (ctx.join('\n\n') || '(no label matches in runtime data)') +
    '\n\n=== MODEL PAIRS ===\n' + ([...pairs].sort().join('\n') || '(none)') +
    '\n\n=== CANDIDATE KEYS ===\n' + ([...keys].sort().join('\n') || '(none)') +
    '\n\n=== DOM/REACT FALLBACK ===\n' + (dom.join('\n\n') || '(no label nodes in DOM — open the model dropdown first)');
}
`

func runProbe(p *rod.Page, js string) (string, error) {
	// Bundle scan can fetch several MB of JS; give it room.
	p = p.Timeout(90 * time.Second)
	res, err := p.Eval(js)
	if err != nil {
		return "", err
	}
	var out string
	if err := res.Value.Unmarshal(&out); err != nil {
		return "", err
	}
	return out, nil
}
