// Drives the labs.google Flow page end-to-end:
//   1. Open / find the project page.
//   2. Click the model picker button (the one with "crop_" + "x" inside).
//   3. Inspect the menu, look for a Veo / video model and select it.
//   4. Type the prompt into the Slate.js editor via CDP Input.insertText.
//   5. Click the bottom "Tạo" arrow_forward button (filtered by y > 600).
//
// While doing this, also captures all aisandbox-pa.googleapis.com traffic
// to a JSONL trace so we can reverse-engineer the API.
//
// Usage:
//   node auto-submit-flow.mjs --ws ws://… --prompt "Một con mèo bay" --out reports/

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
const prompt = args.prompt || 'Một con mèo nhỏ bay qua bầu trời lúc bình minh';
const outDir = path.resolve(args.out || './reports');
const matchHost = 'aisandbox-pa.googleapis.com';

if (!wsEndpoint) { console.error('--ws required'); process.exit(1); }

fs.mkdirSync(outDir, { recursive: true });
const traceFile = path.join(outDir, 'flow-trace.jsonl');
const stream = fs.createWriteStream(traceFile, { flags: 'w' });
const log = (obj) => stream.write(JSON.stringify(obj) + '\n');

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];
  if (!page.url().includes('labs.google')) {
    await page.goto('https://labs.google/fx/vi/tools/flow', { waitUntil: 'domcontentloaded', timeout: 60000 });
    await sleep(2500);
  }

  // ── Network capture ────────────────────────────────────────────────────
  const cdp = await page.target().createCDPSession();
  await cdp.send('Network.enable');
  const requests = new Map();
  cdp.on('Network.requestWillBeSent', (e) => {
    if (!(e.request.url || '').includes(matchHost)) return;
    const r = {
      ts: Date.now(),
      url: e.request.url,
      method: e.request.method,
      headers: Object.fromEntries(
        Object.entries(e.request.headers || {}).filter(([k]) => k.toLowerCase() !== 'authorization'),
      ),
      body: e.request.postData || null,
    };
    requests.set(e.requestId, r);
    log({ kind: 'request', id: e.requestId, ...r });
  });
  cdp.on('Network.responseReceived', (e) => {
    if (!(e.response.url || '').includes(matchHost)) return;
    log({
      kind: 'response', id: e.requestId, url: e.response.url,
      status: e.response.status, mimeType: e.response.mimeType,
    });
  });
  cdp.on('Network.loadingFinished', async (e) => {
    if (!requests.has(e.requestId)) return;
    try {
      const { body, base64Encoded } = await cdp.send('Network.getResponseBody', { requestId: e.requestId });
      const decoded = base64Encoded ? Buffer.from(body, 'base64').toString('utf8') : body;
      log({ kind: 'response_body', id: e.requestId, bodyLen: decoded.length, bodyPreview: decoded.slice(0, 8000) });
    } catch (err) {
      log({ kind: 'response_body_err', id: e.requestId, err: String(err.message || err) });
    }
  });

  const phaseLog = [];
  function step(name, extra = {}) {
    const m = { phase: name, ts: Date.now(), ...extra };
    phaseLog.push(m); log({ kind: 'phase', ...m });
    console.error('PHASE:', name, JSON.stringify(extra));
  }

  step('start', { url: page.url() });

  // ── 1. Open model picker (button with "crop_" + ariaHasPopup="menu") ──
  step('opening_model_picker');
  const pickerHandle = await page.evaluateHandle(() => {
    const btns = Array.from(document.querySelectorAll('button[aria-haspopup="menu"]'));
    return btns.find((b) => /crop_/i.test(b.innerText || '')) || null;
  });
  const pickerOK = await pickerHandle.evaluate((el) => !!el);
  if (!pickerOK) {
    step('picker_not_found');
  } else {
    await pickerHandle.click();
    await sleep(900);
  }

  await page.screenshot({ path: path.join(outDir, '01-picker-open.png') });

  // Inspect menu
  const menu = await page.evaluate(() => {
    const items = Array.from(document.querySelectorAll('[role="menuitem"]')).map((m) => ({
      text: (m.innerText || '').trim().slice(0, 100),
      ariaChecked: m.getAttribute('aria-checked'),
    }));
    const tabs = Array.from(document.querySelectorAll('[role="tab"]')).map((t) => ({
      text: (t.innerText || '').trim().slice(0, 30),
      dataState: t.getAttribute('data-state'),
    }));
    return { items, tabs };
  });
  step('menu_dump', menu);
  await fsp.writeFile(path.join(outDir, '01-picker-menu.json'), JSON.stringify(menu, null, 2));

  // Try select Veo (any item containing "Veo" or "veo_3")
  const selectedVeo = await page.evaluate(() => {
    const items = Array.from(document.querySelectorAll('[role="menuitem"]'));
    const veo = items.find((m) => /veo/i.test(m.innerText || ''));
    if (veo) { veo.click(); return (veo.innerText || '').trim(); }
    return null;
  });
  step('selected_model', { selectedVeo });
  await sleep(800);

  // ── 2. Type prompt via CDP Input.insertText ────────────────────────────
  step('focus_editor');
  await page.evaluate(() => {
    const el = document.querySelector('[contenteditable="true"][role="textbox"], [contenteditable="true"]');
    if (el) el.focus();
  });
  await sleep(200);
  await cdp.send('Input.insertText', { text: prompt });
  step('inserted_prompt', { len: prompt.length });
  await sleep(400);

  await page.screenshot({ path: path.join(outDir, '02-prompt-typed.png') });

  // ── 3. Click submit (arrow_forward button at y > 600) ──────────────────
  step('click_submit');
  const clicked = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    // Match the bottom-row arrow_forward button.
    const candidates = btns.filter((b) => {
      const txt = (b.innerText || '').trim();
      const r = b.getBoundingClientRect();
      return /arrow_forward|Tạo/.test(txt) && r.y > 600 && r.width < 80; // small icon button
    });
    if (!candidates.length) return null;
    candidates[0].click();
    const r = candidates[0].getBoundingClientRect();
    return { text: candidates[0].innerText.trim(), x: Math.round(r.x), y: Math.round(r.y) };
  });
  step('submit_clicked', { clicked });

  // ── 4. Wait for traffic to flow ────────────────────────────────────────
  await sleep(15000);
  await page.screenshot({ path: path.join(outDir, '03-after-submit.png') });

  step('end');
  stream.end();
  console.log(JSON.stringify({ success: true, trace: traceFile, screenshots: ['01-picker-open.png', '02-prompt-typed.png', '03-after-submit.png'].map((n) => path.join(outDir, n)), phases: phaseLog.length }, null, 2));

  await browser.disconnect();
})().catch((err) => {
  console.error(JSON.stringify({ success: false, error: String(err.message || err), stack: err.stack }));
  process.exit(1);
});
