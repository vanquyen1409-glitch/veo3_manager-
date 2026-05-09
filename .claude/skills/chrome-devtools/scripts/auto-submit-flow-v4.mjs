// v4: submit a fresh prompt, then KEEP the network capture running for the
// full poll → success → download cycle (~3 min for veo_3_1_t2v_fast).

import puppeteer from 'puppeteer';
import fs from 'node:fs';
import path from 'node:path';

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, v, i, a) => {
    if (v.startsWith('--')) acc.push([v.slice(2), a[i + 1]]);
    return acc;
  }, []),
);

const wsEndpoint = args.ws;
const prompt = args.prompt || 'A red sports car driving through neon city at night, cinematic';
const outDir = path.resolve(args.out || './reports');
const matchHosts = ['aisandbox-pa.googleapis.com', 'storage.googleapis.com', 'googleusercontent.com'];
const watchMs = parseInt(args.watch || '180000', 10);

if (!wsEndpoint) { console.error('--ws required'); process.exit(1); }

fs.mkdirSync(outDir, { recursive: true });
const traceFile = path.join(outDir, 'flow-trace-v4.jsonl');
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

  function isMatch(url) { return matchHosts.some((h) => url.includes(h)); }

  cdp.on('Network.requestWillBeSent', (e) => {
    if (!isMatch(e.request.url || '')) return;
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
    if (!isMatch(e.response.url || '')) return;
    log({ kind: 'response', id: e.requestId, url: e.response.url, status: e.response.status, mimeType: e.response.mimeType });
  });
  cdp.on('Network.loadingFinished', async (e) => {
    if (!requests.has(e.requestId)) return;
    try {
      const { body, base64Encoded } = await cdp.send('Network.getResponseBody', { requestId: e.requestId });
      const decoded = base64Encoded ? Buffer.from(body, 'base64').toString('utf8') : body;
      log({ kind: 'response_body', id: e.requestId, bodyLen: decoded.length, bodyPreview: decoded.slice(0, 16000) });
    } catch (err) {
      log({ kind: 'response_body_err', id: e.requestId, err: String(err.message || err) });
    }
  });

  function step(name, extra = {}) {
    log({ kind: 'phase', phase: name, ts: Date.now(), ...extra });
    console.error('PHASE:', name, JSON.stringify(extra));
  }

  step('start', { url: page.url() });

  // 1) Clear prompt + type new
  const editor = await page.$('[contenteditable="true"][role="textbox"], [contenteditable="true"]');
  if (editor) {
    await editor.click();
    await page.keyboard.down('Control'); await page.keyboard.press('KeyA'); await page.keyboard.up('Control');
    await page.keyboard.press('Delete');
    await sleep(200);
    await cdp.send('Input.insertText', { text: prompt });
    step('prompt_typed', { len: prompt.length });
  }
  await sleep(400);

  // 2) Submit
  const submitBox = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const cands = btns.filter((b) => {
      const t = (b.innerText || '').trim();
      const r = b.getBoundingClientRect();
      return t.includes('arrow_forward') && r.y > 600 && !b.disabled;
    });
    if (!cands.length) return null;
    const r = cands[0].getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  });
  if (submitBox) {
    await page.mouse.click(submitBox.x, submitBox.y);
    step('submitted', submitBox);
  }

  // 3) Watch for the full poll → success cycle
  step('watching', { durationMs: watchMs });
  await sleep(watchMs);

  step('end');
  stream.end();
  console.log(JSON.stringify({ success: true, trace: traceFile }, null, 2));
  await browser.disconnect();
})().catch((err) => {
  console.error(JSON.stringify({ success: false, error: String(err.message || err) }));
  process.exit(1);
});
