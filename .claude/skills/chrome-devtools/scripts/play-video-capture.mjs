// Click on the first generated video tile, watch network for any mp4 URLs
// (likely googleusercontent.com or storage.googleapis.com).

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
const outDir = path.resolve(args.out || './reports');
fs.mkdirSync(outDir, { recursive: true });
const traceFile = path.join(outDir, 'play-trace.jsonl');
const stream = fs.createWriteStream(traceFile, { flags: 'w' });
const log = (o) => stream.write(JSON.stringify(o) + '\n');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];

  const cdp = await page.target().createCDPSession();
  await cdp.send('Network.enable');
  cdp.on('Network.requestWillBeSent', (e) => {
    const u = e.request.url || '';
    // Watch ALL requests so we can spot the mp4.
    if (/\.mp4|googleusercontent|storage\.googleapis|labs\.google.*\.video|\?alt=media/i.test(u)) {
      log({
        kind: 'request', id: e.requestId, url: u, method: e.request.method,
        type: e.type, initiator: e.initiator?.type,
        headers: Object.fromEntries(Object.entries(e.request.headers || {}).filter(([k]) => k.toLowerCase() !== 'authorization')),
      });
    }
  });
  cdp.on('Network.responseReceived', (e) => {
    const u = e.response.url || '';
    if (/\.mp4|googleusercontent|storage\.googleapis|video/i.test(u)) {
      log({
        kind: 'response', id: e.requestId, url: u,
        status: e.response.status, mimeType: e.response.mimeType,
        contentLength: e.response.headers?.['content-length'],
      });
    }
  });

  // Click first video tile (find a <video> or play_circle button)
  const clicked = await page.evaluate(() => {
    const v = document.querySelector('video');
    if (v) { v.click(); return { kind: 'video', src: v.currentSrc || v.src }; }
    const playBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      /play_circle|play_arrow/i.test(b.innerText || ''));
    if (playBtn) { playBtn.click(); return { kind: 'play_btn' }; }
    return null;
  });
  log({ kind: 'phase', phase: 'clicked', detail: clicked });

  await sleep(8000);
  await page.screenshot({ path: path.join(outDir, 'play-state.png') });

  log({ kind: 'end' });
  stream.end();
  console.log(JSON.stringify({ success: true, trace: traceFile, clicked }, null, 2));
  await browser.disconnect();
})().catch((e) => { console.error(JSON.stringify({ success: false, error: String(e.message) })); process.exit(1); });
