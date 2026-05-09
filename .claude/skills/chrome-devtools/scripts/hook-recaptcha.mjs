// Monkey-patch grecaptcha.enterprise.execute on the page so we can record
// what siteKey + action labs.google itself uses when the user clicks Create.
// Usage: run, click submit in the UI, check the report.

import puppeteer from 'puppeteer';
import fs from 'node:fs';

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, v, i, a) => {
    if (v.startsWith('--')) acc.push([v.slice(2), a[i + 1]]);
    return acc;
  }, []),
);
const wsEndpoint = args.ws;
const watchMs = parseInt(args.watch || '60000', 10);

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];

  // Attach hooks
  await page.evaluate(() => {
    if (window.__recapHookInstalled) return;
    window.__recapHookInstalled = true;
    window.__recapCalls = [];
    const ge = window.grecaptcha && window.grecaptcha.enterprise;
    if (!ge) return;
    const orig = ge.execute.bind(ge);
    ge.execute = function (siteKey, opts) {
      try {
        window.__recapCalls.push({ ts: Date.now(), siteKey, opts: { ...(opts || {}) } });
      } catch {}
      return orig(siteKey, opts);
    };
  });

  console.error('[hook] installed; waiting', watchMs, 'ms — please click Create in the page');
  await new Promise((r) => setTimeout(r, watchMs));

  const calls = await page.evaluate(() => window.__recapCalls || []);
  console.log(JSON.stringify({ success: true, calls }, null, 2));
  await browser.disconnect();
})().catch((e) => { console.error(JSON.stringify({ error: String(e.message) })); process.exit(1); });
