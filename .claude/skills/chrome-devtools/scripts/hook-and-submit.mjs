// Install recaptcha hook + auto-click submit + capture the call.
import puppeteer from 'puppeteer';

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, v, i, a) => {
    if (v.startsWith('--')) acc.push([v.slice(2), a[i + 1]]);
    return acc;
  }, []),
);
const wsEndpoint = args.ws;
const prompt = args.prompt || 'Hook test prompt';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

(async () => {
  const browser = await puppeteer.connect({ browserWSEndpoint: wsEndpoint, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages.find((p) => p.url().includes('labs.google')) || pages[0];

  // 1) Install hook
  await page.evaluate(() => {
    window.__recapCalls = window.__recapCalls || [];
    const ge = window.grecaptcha && window.grecaptcha.enterprise;
    if (!ge || ge.__patched) return;
    const orig = ge.execute.bind(ge);
    ge.execute = function (siteKey, opts) {
      window.__recapCalls.push({ ts: Date.now(), siteKey, opts: opts ? { ...opts } : null });
      return orig(siteKey, opts);
    };
    ge.__patched = true;
  });

  // 2) Clear editor + type prompt
  const cdp = await page.target().createCDPSession();
  const editor = await page.$('[contenteditable="true"][role="textbox"], [contenteditable="true"]');
  if (editor) {
    await editor.click();
    await page.keyboard.down('Control'); await page.keyboard.press('KeyA'); await page.keyboard.up('Control');
    await page.keyboard.press('Delete');
    await sleep(150);
    await cdp.send('Input.insertText', { text: prompt });
  }
  await sleep(400);

  // 3) Click submit
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
  }

  // 4) Wait for grecaptcha call
  await sleep(4000);
  const calls = await page.evaluate(() => window.__recapCalls || []);
  console.log(JSON.stringify({ success: true, calls }, null, 2));
  await browser.disconnect();
})().catch((e) => { console.error(JSON.stringify({ error: String(e.message) })); process.exit(1); });
