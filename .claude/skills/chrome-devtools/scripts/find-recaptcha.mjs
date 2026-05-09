// Discover the reCAPTCHA Enterprise site key + verify we can execute it.
import puppeteer from 'puppeteer';

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, v, i, a) => {
    if (v.startsWith('--')) acc.push([v.slice(2), a[i + 1]]);
    return acc;
  }, []),
);
const wsEndpoint = args.ws;
if (!wsEndpoint) { console.error('--ws required'); process.exit(1); }

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];

  const info = await page.evaluate(async () => {
    const out = { hasGrecaptcha: typeof window.grecaptcha !== 'undefined', enterprise: false, scripts: [], siteKeys: [], action: null };
    out.enterprise = !!(window.grecaptcha && window.grecaptcha.enterprise);

    // Recaptcha script URLs
    out.scripts = Array.from(document.querySelectorAll('script[src*="recaptcha"]')).map(s => s.src);
    // Most enterprise scripts include ?render=<sitekey>
    for (const s of out.scripts) {
      const m = s.match(/[?&]render=([^&]+)/);
      if (m) out.siteKeys.push(m[1]);
    }

    // Walk window.___grecaptcha_cfg for sitekey too
    try {
      const cfg = window.___grecaptcha_cfg || window.__recaptcha_api;
      const seen = new Set();
      const visit = (n, depth = 0) => {
        if (depth > 8 || !n || typeof n !== 'object' || seen.has(n)) return;
        seen.add(n);
        for (const k of Object.keys(n)) {
          if (k === 'sitekey' && typeof n[k] === 'string') out.siteKeys.push(n[k]);
          if (typeof n[k] === 'object') visit(n[k], depth + 1);
        }
      };
      visit(cfg);
    } catch {}
    out.siteKeys = Array.from(new Set(out.siteKeys));

    // Try to actually execute and get a token
    if (out.enterprise && out.siteKeys.length) {
      try {
        const sk = out.siteKeys[0];
        const tok = await window.grecaptcha.enterprise.execute(sk, { action: 'BATCH_ASYNC_GENERATE_VIDEO' });
        out.tokenLen = tok.length;
        out.tokenPreview = tok.slice(0, 30);
      } catch (e) { out.executeError = String(e); }
    }
    return out;
  });

  console.log(JSON.stringify(info, null, 2));
  await browser.disconnect();
})().catch((e) => { console.error(JSON.stringify({ error: String(e.message) })); process.exit(1); });
