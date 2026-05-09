// Long-running watcher: connects to user's Chrome via CDP, attaches to the
// currently-open Flow page, and logs every aisandbox-pa.googleapis.com request
// (URL, method, headers minus auth, body, response status, response body
// preview) to a JSONL file. Stop with Ctrl+C.
//
// Usage:
//   node watch-labs-api.mjs --ws ws://… --out path/to/api-trace.jsonl

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
const outFile = path.resolve(args.out || './api-trace.jsonl');
const matchHost = args.host || 'aisandbox-pa.googleapis.com';
const durationMs = parseInt(args.duration || '60000', 10);

if (!wsEndpoint) { console.error('--ws required'); process.exit(1); }

fs.mkdirSync(path.dirname(outFile), { recursive: true });
const stream = fs.createWriteStream(outFile, { flags: 'a' });
const log = (obj) => stream.write(JSON.stringify(obj) + '\n');

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];

  // Hook ALL existing pages + future pages to enable network domain.
  async function attach(p) {
    const cdp = await p.target().createCDPSession();
    await cdp.send('Network.enable');

    const requests = new Map();

    cdp.on('Network.requestWillBeSent', (e) => {
      const url = e.request.url || '';
      if (!url.includes(matchHost)) return;
      requests.set(e.requestId, {
        ts: Date.now(),
        url,
        method: e.request.method,
        // Drop authorization for the trace; we only need shape.
        headers: Object.fromEntries(
          Object.entries(e.request.headers || {}).filter(([k]) => k.toLowerCase() !== 'authorization'),
        ),
        body: e.request.postData || null,
      });
      log({ kind: 'request', id: e.requestId, ...requests.get(e.requestId) });
    });

    cdp.on('Network.responseReceived', (e) => {
      const url = e.response.url || '';
      if (!url.includes(matchHost)) return;
      log({
        kind: 'response',
        id: e.requestId,
        url,
        status: e.response.status,
        statusText: e.response.statusText,
        mimeType: e.response.mimeType,
      });
    });

    cdp.on('Network.loadingFinished', async (e) => {
      if (!requests.has(e.requestId)) return;
      try {
        const { body, base64Encoded } = await cdp.send('Network.getResponseBody', { requestId: e.requestId });
        const decoded = base64Encoded ? Buffer.from(body, 'base64').toString('utf8') : body;
        log({ kind: 'response_body', id: e.requestId, bodyPreview: decoded.slice(0, 4000), bodyLen: decoded.length });
      } catch (err) {
        log({ kind: 'response_body_err', id: e.requestId, err: String(err.message || err) });
      }
    });

    cdp.on('Network.loadingFailed', (e) => {
      if (!requests.has(e.requestId)) return;
      log({ kind: 'failed', id: e.requestId, error: e.errorText });
    });
  }

  await attach(page);

  console.log(JSON.stringify({ success: true, attached: page.url(), out: outFile, watchingFor: matchHost, durationMs }));

  // Keep alive for the requested duration. Disconnect cleanly so user's
  // Chrome stays running.
  setTimeout(async () => {
    log({ kind: 'end', ts: Date.now() });
    stream.end();
    await browser.disconnect();
    process.exit(0);
  }, durationMs);
})().catch((err) => {
  console.error(JSON.stringify({ success: false, error: String(err.message || err) }));
  process.exit(1);
});
