// Open the labs.google media redirect URL and dump everything that happens.
import puppeteer from 'puppeteer';

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, v, i, a) => {
    if (v.startsWith('--')) acc.push([v.slice(2), a[i + 1]]);
    return acc;
  }, []),
);

const wsEndpoint = args.ws;
const mediaId = args.media || '762d5731-ab17-4a33-b4af-fcff4a95c716';

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const page = await browser.newPage();
  const cdp = await page.target().createCDPSession();
  await cdp.send('Network.enable');

  const events = [];
  cdp.on('Network.requestWillBeSent', (e) => events.push({ kind: 'req', url: e.request.url, method: e.request.method, redirect: e.redirectResponse?.url }));
  cdp.on('Network.responseReceived', (e) => events.push({ kind: 'resp', url: e.response.url, status: e.response.status, mimeType: e.response.mimeType }));

  const url = `https://labs.google/fx/api/trpc/media.getMediaUrlRedirect?name=${mediaId}`;
  console.error('[navigate]', url);
  try {
    await page.goto(url, { waitUntil: 'networkidle2', timeout: 30000 });
  } catch (e) {
    console.error('[goto err]', e.message);
  }

  await new Promise(r => setTimeout(r, 3000));
  const finalURL = page.url();
  const html = await page.content();

  console.log(JSON.stringify({
    success: true,
    finalURL,
    htmlLen: html.length,
    htmlPreview: html.slice(0, 600),
    events: events.slice(0, 25),
  }, null, 2));

  await page.close();
  await browser.disconnect();
})().catch((e) => { console.error(JSON.stringify({ error: String(e.message) })); process.exit(1); });
