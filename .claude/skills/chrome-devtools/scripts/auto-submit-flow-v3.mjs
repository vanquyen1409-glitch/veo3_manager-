// v3: use page.click() with element handles (real OS-level mouse events)
// instead of evaluate(()=>el.click()) — Radix/MUI menus close on
// outside-detected synthetic clicks. Also clear the editor first.

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
const prompt = args.prompt || 'A small cat flying through morning sky, golden light';
const outDir = path.resolve(args.out || './reports');
const matchHost = 'aisandbox-pa.googleapis.com';

if (!wsEndpoint) { console.error('--ws required'); process.exit(1); }

fs.mkdirSync(outDir, { recursive: true });
const traceFile = path.join(outDir, 'flow-trace-v3.jsonl');
const stream = fs.createWriteStream(traceFile, { flags: 'w' });
const log = (obj) => stream.write(JSON.stringify(obj) + '\n');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];

  const cdp = await page.target().createCDPSession();
  await cdp.send('Network.enable');
  const requests = new Map();
  cdp.on('Network.requestWillBeSent', (e) => {
    if (!(e.request.url || '').includes(matchHost)) return;
    const r = {
      ts: Date.now(), url: e.request.url, method: e.request.method,
      headers: { ...(e.request.headers || {}) },
      body: e.request.postData || null,
    };
    if (r.headers.Authorization) r.headers.Authorization = r.headers.Authorization.slice(0, 25) + '…<redacted>';
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
      log({ kind: 'response_body', id: e.requestId, bodyLen: decoded.length, bodyPreview: decoded.slice(0, 16000) });
    } catch { /* ignore */ }
  });

  function step(name, extra = {}) {
    log({ kind: 'phase', phase: name, ts: Date.now(), ...extra });
    console.error('PHASE:', name, JSON.stringify(extra));
  }

  step('start', { url: page.url() });

  // ── Step 0: Clear editor (Ctrl+A + Delete) ──────────────────────────────
  const editorH = await page.$('[contenteditable="true"][role="textbox"], [contenteditable="true"]');
  if (editorH) {
    await editorH.click();
    await page.keyboard.down('Control');
    await page.keyboard.press('KeyA');
    await page.keyboard.up('Control');
    await page.keyboard.press('Delete');
    step('editor_cleared');
  }
  await sleep(300);

  // ── Step 1: Click model picker via REAL puppeteer click ────────────────
  // We can't query by xpath text easily, so we get the bbox via evaluate
  // and call page.mouse.click(x, y) — which Radix sees as a real user event.
  const pickerBox = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll('button[aria-haspopup="menu"]'))
      .find((b) => /crop_/i.test(b.innerText || ''));
    if (!btn) return null;
    const r = btn.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  });
  step('picker_box', pickerBox);
  if (pickerBox) {
    await page.mouse.click(pickerBox.x, pickerBox.y);
    await sleep(800);
  }
  await page.screenshot({ path: path.join(outDir, '01-picker-opened.png') });

  // ── Step 2: Click "Video" tab via mouse coords ─────────────────────────
  const videoTabBox = await page.evaluate(() => {
    const tabs = Array.from(document.querySelectorAll('[role="tab"]'));
    const v = tabs.find((t) => /Video/i.test(t.innerText || ''));
    if (!v) return null;
    const r = v.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2, text: v.innerText.trim() };
  });
  step('video_tab_box', videoTabBox);
  if (videoTabBox) {
    await page.mouse.click(videoTabBox.x, videoTabBox.y);
    await sleep(900);
  }
  await page.screenshot({ path: path.join(outDir, '02-video-tab.png') });

  // ── Step 3: Dump menu after switching to Video ─────────────────────────
  const videoMenu = await page.evaluate(() => ({
    items: Array.from(document.querySelectorAll('[role="menuitem"]')).map((m) => ({
      text: (m.innerText || '').trim().slice(0, 200),
      ariaChecked: m.getAttribute('aria-checked'),
    })),
    tabs: Array.from(document.querySelectorAll('[role="tab"]')).map((t) => ({
      text: (t.innerText || '').trim().slice(0, 30),
      dataState: t.getAttribute('data-state'),
    })),
    radios: Array.from(document.querySelectorAll('[role="radio"], [role="radiogroup"] *')).map((r) => ({
      role: r.getAttribute('role'),
      text: (r.innerText || '').trim().slice(0, 80),
      ariaChecked: r.getAttribute('aria-checked'),
    })).slice(0, 20),
  }));
  step('menu_after_video', videoMenu);
  await fsp.writeFile(path.join(outDir, 'video-menu.json'), JSON.stringify(videoMenu, null, 2));

  // ── Step 4: Find and click a Veo menuitem (real click via mouse) ───────
  const veoBox = await page.evaluate(() => {
    const items = Array.from(document.querySelectorAll('[role="menuitem"]'));
    const veo = items.find((m) => /veo/i.test(m.innerText || ''));
    if (!veo) return null;
    const r = veo.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2, text: veo.innerText.trim() };
  });
  step('veo_box', veoBox);
  if (veoBox) {
    await page.mouse.click(veoBox.x, veoBox.y);
    await sleep(700);
  }

  // Close any leftover menu
  await page.keyboard.press('Escape');
  await sleep(300);
  await page.screenshot({ path: path.join(outDir, '03-after-veo.png') });

  // ── Step 5: Type prompt ────────────────────────────────────────────────
  const editor2 = await page.$('[contenteditable="true"][role="textbox"], [contenteditable="true"]');
  if (editor2) {
    await editor2.click();
    await sleep(150);
    // Clear again
    await page.keyboard.down('Control');
    await page.keyboard.press('KeyA');
    await page.keyboard.up('Control');
    await page.keyboard.press('Delete');
    await sleep(150);
    await cdp.send('Input.insertText', { text: prompt });
    step('prompt_typed', { len: prompt.length });
  }
  await sleep(500);
  await page.screenshot({ path: path.join(outDir, '04-prompt-typed.png') });

  // ── Step 6: Click submit (arrow_forward) ───────────────────────────────
  const submitBox = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const cands = btns.filter((b) => {
      const t = (b.innerText || '').trim();
      const r = b.getBoundingClientRect();
      return t.includes('arrow_forward') && r.y > 600 && !b.disabled;
    });
    if (!cands.length) return null;
    const r = cands[0].getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2, text: cands[0].innerText.trim() };
  });
  step('submit_box', submitBox);
  if (submitBox) {
    await page.mouse.click(submitBox.x, submitBox.y);
    step('submit_clicked');
  }

  // ── Step 7: Wait for the API call to fire and complete ─────────────────
  await sleep(25000);
  await page.screenshot({ path: path.join(outDir, '05-after-submit.png') });

  step('end');
  stream.end();
  console.log(JSON.stringify({ success: true, trace: traceFile, outDir }, null, 2));
  await browser.disconnect();
})().catch((err) => {
  console.error(JSON.stringify({ success: false, error: String(err.message || err), stack: err.stack }));
  process.exit(1);
});
