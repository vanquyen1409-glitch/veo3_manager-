// Show response body for any request whose URL matches a substring.
import fs from 'node:fs';
const file = process.argv[2];
const filter = process.argv[3] || '';
if (!file) { console.error('usage: node extract-by-url.mjs <jsonl> <url-substr>'); process.exit(1); }

const lines = fs.readFileSync(file, 'utf8').split('\n').filter(Boolean);
const reqMap = new Map();
const responses = [];

for (const line of lines) {
  try {
    const o = JSON.parse(line);
    if (o.kind === 'request') reqMap.set(o.id, o);
    if (o.kind === 'response' && o.url.includes(filter)) responses.push(o);
    if (o.kind === 'response_body') {
      const req = reqMap.get(o.id);
      if (req && req.url.includes(filter)) {
        console.log('=== Request:', req.method, req.url);
        if (req.body) console.log('Request body (first 1000):', String(req.body).slice(0, 1000));
        console.log('Response body (first 4000):');
        console.log(o.bodyPreview.slice(0, 4000));
        console.log('---\n');
      }
    }
  } catch {}
}
