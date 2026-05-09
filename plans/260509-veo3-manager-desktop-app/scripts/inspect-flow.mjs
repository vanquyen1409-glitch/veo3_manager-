// Connect to user's running Chrome via CDP, find or open the labs.google
// Flow page, take a screenshot, dump DOM markers we'll need to reverse-
// engineer the prompt → submit flow.
//
// Usage:
//   node inspect-flow.mjs --ws ws://127.0.0.1:9222/devtools/browser/<id> \
//                        --out ./reports/flow-current.png

import puppeteer from 'puppeteer';
import fs from 'node:fs/promises';
import path from 'node:path';

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, v, i, a) => {
    if (v.startsWith('--')) acc.push([v.slice(2), a[i + 1]]);
    return acc;
  }, []),
);

const wsEndpoint = args.ws;
const outPath = path.resolve(args.out || './flow-current.png');
const targetURL = args.url || 'https://labs.google/fx/vi/tools/flow';

if (!wsEndpoint) {
  console.error(JSON.stringify({ success: false, error: 'missing --ws' }));
  process.exit(1);
}

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });

  // 1) Find a Flow tab if one is already open; otherwise navigate the first tab.
  const pages = await browser.pages();
  let page = pages.find((p) => p.url().includes('labs.google'));
  if (!page) {
    page = pages[0] || (await browser.newPage());
    await page.goto(targetURL, { waitUntil: 'domcontentloaded', timeout: 60000 });
  } else {
    // If the existing tab is on a different Flow path, navigate to the Vietnamese tools/flow.
    if (!page.url().includes('/tools/flow')) {
      await page.goto(targetURL, { waitUntil: 'domcontentloaded', timeout: 60000 });
    }
  }

  // 2) Wait a bit for client-side render.
  await new Promise((r) => setTimeout(r, 2500));

  // 3) Screenshot.
  await fs.mkdir(path.dirname(outPath), { recursive: true });
  await page.screenshot({ path: outPath, fullPage: false });

  // 4) Pull DOM markers needed for selector engineering.
  const info = await page.evaluate(() => {
    const out = { url: location.href, title: document.title };

    // __NEXT_DATA__ pageProps keys (token detection).
    try {
      const el = document.getElementById('__NEXT_DATA__');
      if (el) {
        const data = JSON.parse(el.textContent || '{}');
        out.nextDataKeys = Object.keys(data?.props?.pageProps || {});
        const session = data?.props?.pageProps?.session || {};
        out.sessionKeys = Object.keys(session);
        out.sessionUserKeys = Object.keys(session.user || {});
        // peek first 30 chars of any token-looking field
        for (const k of out.sessionKeys) {
          const v = session[k];
          if (typeof v === 'string' && v.length > 50) {
            out['session.' + k + '_preview'] = v.slice(0, 30) + '…(' + v.length + ')';
          }
        }
      }
    } catch (e) { out.nextDataError = String(e); }

    // Buttons: text + position (filters CreateButtonMinY = 680).
    out.buttons = Array.from(document.querySelectorAll('button')).map((b) => {
      const r = b.getBoundingClientRect();
      return {
        text: (b.innerText || b.textContent || '').trim().slice(0, 60),
        x: Math.round(r.x), y: Math.round(r.y),
        w: Math.round(r.width), h: Math.round(r.height),
        ariaHasPopup: b.getAttribute('aria-haspopup') || null,
        disabled: b.disabled,
      };
    }).filter((b) => b.text);

    // Editable contenteditable / textboxes (Slate.js prompt input).
    out.editors = Array.from(document.querySelectorAll('[contenteditable="true"], textarea, input[type="text"]')).slice(0, 10).map((el) => ({
      tag: el.tagName,
      role: el.getAttribute('role'),
      contenteditable: el.getAttribute('contenteditable'),
      placeholder: el.getAttribute('placeholder') || el.getAttribute('data-placeholder'),
      ariaLabel: el.getAttribute('aria-label'),
    }));

    // Tabs (aspect ratio + output count).
    out.tabs = Array.from(document.querySelectorAll('[role="tab"]')).slice(0, 12).map((t) => ({
      text: (t.innerText || '').trim().slice(0, 30),
      dataState: t.getAttribute('data-state'),
    }));

    // Menu items (model picker).
    out.menuitems = Array.from(document.querySelectorAll('[role="menuitem"]')).slice(0, 12).map((m) => ({
      text: (m.innerText || '').trim().slice(0, 80),
    }));

    // Cookies in JS-accessible scope (just a count + names; content not exfiltrated).
    out.cookieNames = document.cookie.split(';').map((c) => c.split('=')[0].trim()).filter(Boolean);
    return out;
  });

  // Dump
  await fs.writeFile(
    outPath.replace(/\.png$/, '.json'),
    JSON.stringify(info, null, 2),
    'utf8',
  );

  console.log(JSON.stringify({
    success: true,
    screenshot: outPath,
    dump: outPath.replace(/\.png$/, '.json'),
    url: info.url,
    title: info.title,
    pagePropKeys: info.nextDataKeys,
    sessionKeys: info.sessionKeys,
  }, null, 2));

  await browser.disconnect();
})().catch((err) => {
  console.error(JSON.stringify({ success: false, error: String(err.message || err), stack: err.stack }));
  process.exit(1);
});
