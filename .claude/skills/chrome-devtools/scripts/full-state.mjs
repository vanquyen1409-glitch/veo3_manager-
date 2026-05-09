// Full-page screenshot + thorough DOM dump for debugging the labs.google
// UI state after automation has been driving the page.
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
const outDir = path.resolve(args.out || './diag');
const navigate = args.url; // optional override

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];

  if (navigate) {
    await page.goto(navigate, { waitUntil: 'domcontentloaded', timeout: 30000 });
    await new Promise(r => setTimeout(r, 2500));
  }

  await fs.mkdir(outDir, { recursive: true });

  // Screenshot full-page (whole scroll height)
  const fullPath = path.join(outDir, 'full.png');
  await page.screenshot({ path: fullPath, fullPage: true });

  // Viewport-only screenshot
  const vPath = path.join(outDir, 'viewport.png');
  await page.screenshot({ path: vPath, fullPage: false });

  const info = await page.evaluate(() => {
    const out = { url: location.href, title: document.title, scrollY: window.scrollY, vw: innerWidth, vh: innerHeight, sh: document.documentElement.scrollHeight };

    out.buttons = Array.from(document.querySelectorAll('button')).map((b) => {
      const r = b.getBoundingClientRect();
      return {
        text: (b.innerText || '').trim().slice(0, 80),
        x: Math.round(r.x), y: Math.round(r.y),
        w: Math.round(r.width), h: Math.round(r.height),
        visible: r.width > 0 && r.height > 0 && r.bottom > 0 && r.right > 0,
        ariaLabel: b.getAttribute('aria-label'),
        ariaHasPopup: b.getAttribute('aria-haspopup'),
        disabled: b.disabled,
      };
    }).filter((b) => b.text || b.ariaLabel);

    out.editors = Array.from(document.querySelectorAll('[contenteditable="true"], textarea, input[type="text"]')).slice(0, 15).map((el) => {
      const r = el.getBoundingClientRect();
      return {
        tag: el.tagName, role: el.getAttribute('role'),
        contenteditable: el.getAttribute('contenteditable'),
        placeholder: el.getAttribute('placeholder') || el.getAttribute('data-placeholder'),
        ariaLabel: el.getAttribute('aria-label'),
        x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
        visible: r.width > 0 && r.height > 0,
      };
    });

    out.headings = Array.from(document.querySelectorAll('h1, h2, h3, [role="heading"]')).slice(0, 20).map((h) => (h.innerText || '').trim().slice(0, 100));
    out.bodyTextHead = (document.body.innerText || '').slice(0, 800);
    return out;
  });

  await fs.writeFile(path.join(outDir, 'state.json'), JSON.stringify(info, null, 2));
  console.log(JSON.stringify({ success: true, full: fullPath, viewport: vPath, dump: path.join(outDir, 'state.json'), url: info.url }, null, 2));

  await browser.disconnect();
})().catch((e) => { console.error(JSON.stringify({ error: String(e.message) })); process.exit(1); });
