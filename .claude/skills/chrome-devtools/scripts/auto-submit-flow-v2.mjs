// v2: switch to Video tab + Veo model + submit via the arrow_forward bottom
// button. Capture full request headers (including Authorization) so we can
// confirm how the API authenticates.

import puppeteer from 'puppeteer';
import fs from 'node:fs';
import fsp from 'node:fs/promises';
import path from 'node:path';

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, v, i, a) => {
    if (v.startsWith('--')) acc.push([v.slice(2), a[i + 1]]);
    return acc;
  }, []),
);

const wsEndpoint = args.ws;
const prompt = args.prompt || 'Một con mèo nhỏ bay qua bầu trời lúc bình minh, ánh sáng vàng dịu';
const outDir = path.resolve(args.out || './reports');
const matchHost = 'aisandbox-pa.googleapis.com';

if (!wsEndpoint) { console.error('--ws required'); process.exit(1); }

fs.mkdirSync(outDir, { recursive: true });
const traceFile = path.join(outDir, 'flow-trace-v2.jsonl');
const stream = fs.createWriteStream(traceFile, { flags: 'w' });
const log = (obj) => stream.write(JSON.stringify(obj) + '\n');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];

  // Network capture (FULL headers — token redacted only at output time)
  const cdp = await page.target().createCDPSession();
  await cdp.send('Network.enable');
  const requests = new Map();
  cdp.on('Network.requestWillBeSent', (e) => {
    if (!(e.request.url || '').includes(matchHost)) return;
    const r = {
      ts: Date.now(),
      url: e.request.url,
      method: e.request.method,
      headers: { ...(e.request.headers || {}) },
      body: e.request.postData || null,
    };
    // Redact bearer for trace file but keep prefix to confirm shape.
    if (r.headers.Authorization) {
      r.headers.Authorization = r.headers.Authorization.slice(0, 20) + '…<redacted>';
    }
    requests.set(e.requestId, r);
    log({ kind: 'request', id: e.requestId, ...r });
  });
  cdp.on('Network.responseReceived', (e) => {
    if (!(e.response.url || '').includes(matchHost)) return;
    log({ kind: 'response', id: e.requestId, url: e.response.url, status: e.response.status });
  });
  cdp.on('Network.loadingFinished', async (e) => {
    if (!requests.has(e.requestId)) return;
    try {
      const { body, base64Encoded } = await cdp.send('Network.getResponseBody', { requestId: e.requestId });
      const decoded = base64Encoded ? Buffer.from(body, 'base64').toString('utf8') : body;
      log({ kind: 'response_body', id: e.requestId, bodyLen: decoded.length, bodyPreview: decoded.slice(0, 12000) });
    } catch (err) { /* ignore */ }
  });

  function step(name, extra = {}) {
    log({ kind: 'phase', phase: name, ts: Date.now(), ...extra });
    console.error('PHASE:', name, JSON.stringify(extra));
  }

  step('start', { url: page.url() });

  // 1) Open model picker
  const opened = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll('button[aria-haspopup="menu"]'))
      .find((b) => /crop_/i.test(b.innerText || ''));
    if (btn) { btn.click(); return true; }
    return false;
  });
  step('picker_opened', { opened });
  await sleep(900);

  // 2) Click "Video" tab inside the menu
  const videoTab = await page.evaluate(() => {
    const tabs = Array.from(document.querySelectorAll('[role="tab"]'));
    const v = tabs.find((t) => /play_circle|video/i.test(t.innerText || ''));
    if (v) { v.click(); return (v.innerText || '').trim(); }
    return null;
  });
  step('video_tab_clicked', { videoTab });
  await sleep(700);

  // 3) Dump menu after switching to Video
  const videoMenu = await page.evaluate(() => {
    return {
      items: Array.from(document.querySelectorAll('[role="menuitem"]')).map((m) => ({
        text: (m.innerText || '').trim().slice(0, 100),
        ariaChecked: m.getAttribute('aria-checked'),
      })),
      tabs: Array.from(document.querySelectorAll('[role="tab"]')).map((t) => ({
        text: (t.innerText || '').trim().slice(0, 30),
        dataState: t.getAttribute('data-state'),
      })),
    };
  });
  step('menu_after_video', videoMenu);
  await fsp.writeFile(path.join(outDir, 'video-menu.json'), JSON.stringify(videoMenu, null, 2));
  await page.screenshot({ path: path.join(outDir, '01-video-menu.png') });

  // 4) Try select a Veo menuitem
  const veoSelected = await page.evaluate(() => {
    const items = Array.from(document.querySelectorAll('[role="menuitem"]'));
    const veo = items.find((m) => /veo/i.test(m.innerText || ''));
    if (veo) { veo.click(); return (veo.innerText || '').trim(); }
    return null;
  });
  step('veo_selected', { veoSelected });
  await sleep(900);

  // 5) Close menu by pressing Escape (so submit button is reachable).
  await page.keyboard.press('Escape');
  await sleep(400);

  await page.screenshot({ path: path.join(outDir, '02-after-veo-select.png') });

  // 6) Type prompt via CDP Input.insertText
  await page.evaluate(() => {
    const el = document.querySelector('[contenteditable="true"][role="textbox"], [contenteditable="true"]');
    if (el) el.focus();
  });
  await sleep(200);
  await cdp.send('Input.insertText', { text: prompt });
  step('prompt_typed', { len: prompt.length });
  await sleep(400);

  await page.screenshot({ path: path.join(outDir, '03-prompt-typed.png') });

  // 7) Click submit button — the ARROW_FORWARD one at the bottom (right of model picker)
  const clicked = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    // arrow_forward is a Material Symbols ligature inside the button text.
    // Filter: must contain "arrow_forward", at the bottom (y > 600), small icon.
    const cands = btns.filter((b) => {
      const t = (b.innerText || '').trim();
      const r = b.getBoundingClientRect();
      return t.includes('arrow_forward') && r.y > 600;
    });
    if (!cands.length) return { ok: false, count: 0 };
    cands[0].click();
    const r = cands[0].getBoundingClientRect();
    return { ok: true, x: Math.round(r.x), y: Math.round(r.y), text: cands[0].innerText.trim() };
  });
  step('submit_clicked', clicked);

  // 8) Wait for traffic
  await sleep(20000);
  await page.screenshot({ path: path.join(outDir, '04-after-submit.png') });

  step('end');
  stream.end();
  console.log(JSON.stringify({
    success: true, trace: traceFile, outDir,
    videoTab, veoSelected, clicked,
  }, null, 2));

  await browser.disconnect();
})().catch((err) => {
  console.error(JSON.stringify({ success: false, error: String(err.message || err), stack: err.stack }));
  process.exit(1);
});
