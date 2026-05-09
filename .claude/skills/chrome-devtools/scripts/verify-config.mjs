// Hit the running VEO3 Manager via its bound HTTP routes is not possible
// (Wails IPC), so we directly exercise the labs.google API the same way
// the app's queue worker would (browser-context fetch from the user's
// signed-in Chrome). Submits 1 prompt with the new settings and prints
// the request body that hits aisandbox-pa for verification.
import puppeteer from 'puppeteer';

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, v, i, a) => {
    if (v.startsWith('--')) acc.push([v.slice(2), a[i + 1]]);
    return acc;
  }, []),
);
const wsEndpoint = args.ws;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];

  const cdp = await page.target().createCDPSession();
  await cdp.send('Network.enable');
  let captured = null;
  cdp.on('Network.requestWillBeSent', (e) => {
    if ((e.request.url || '').includes('batchAsyncGenerateVideoText') && e.request.method === 'POST') {
      captured = { url: e.request.url, body: e.request.postData };
    }
  });

  // Run one minimal submit through the page's own UI
  const editor = await page.$('[contenteditable="true"][role="textbox"], [contenteditable="true"]');
  if (editor) {
    await editor.click();
    await page.keyboard.down('Control'); await page.keyboard.press('KeyA'); await page.keyboard.up('Control');
    await page.keyboard.press('Delete');
    await sleep(100);
    await cdp.send('Input.insertText', { text: 'Verify config 16:9 x1' });
  }
  await sleep(300);

  const submitBox = await page.evaluate(() => {
    const cands = Array.from(document.querySelectorAll('button')).filter((b) => {
      const t = (b.innerText || '').trim();
      const r = b.getBoundingClientRect();
      return t.includes('arrow_forward') && r.y > 600 && !b.disabled;
    });
    if (!cands.length) return null;
    const r = cands[0].getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  });
  if (submitBox) await page.mouse.click(submitBox.x, submitBox.y);

  await sleep(2000);
  console.log(JSON.stringify({
    success: !!captured,
    url: captured?.url,
    body: captured?.body ? JSON.parse(captured.body) : null,
  }, null, 2));
  await browser.disconnect();
})().catch((e) => { console.error(JSON.stringify({ error: String(e.message) })); process.exit(1); });
