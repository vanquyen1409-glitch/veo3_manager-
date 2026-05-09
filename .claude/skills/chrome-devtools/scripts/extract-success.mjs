// Pretty-print all SUCCESSFUL response bodies + the unique Network URLs
// from a flow-trace JSONL file. Used to nail down the download flow.
import fs from 'node:fs';

const file = process.argv[2];
if (!file) { console.error('usage: node extract-success.mjs <jsonl>'); process.exit(1); }

const lines = fs.readFileSync(file, 'utf8').split('\n').filter(Boolean);
const bodies = [];
for (const line of lines) {
  try {
    const o = JSON.parse(line);
    if (o.kind === 'response_body' && o.bodyPreview && o.bodyPreview.includes('STATUS_SUCCESSFUL')) {
      bodies.push(o);
    }
  } catch {}
}

console.log('=== Found', bodies.length, 'SUCCESSFUL response bodies ===\n');
for (const [i, b] of bodies.entries()) {
  console.log('--- Body', i, '(id', b.id, ', len', b.bodyLen, ') ---');
  try {
    const parsed = JSON.parse(b.bodyPreview);
    // Find any URL-looking fields anywhere in the structure.
    const urls = [];
    const visit = (n, path = '$') => {
      if (n == null) return;
      if (typeof n === 'string' && /^https?:\/\//.test(n)) urls.push({ path, value: n });
      if (Array.isArray(n)) n.forEach((v, i) => visit(v, `${path}[${i}]`));
      else if (typeof n === 'object') for (const k of Object.keys(n)) visit(n[k], `${path}.${k}`);
    };
    visit(parsed);
    console.log('URLs found:');
    for (const u of urls) console.log(' ', u.path, '=', u.value);
    console.log('\nFull JSON:');
    console.log(JSON.stringify(parsed, null, 2).slice(0, 8000));
  } catch (e) {
    console.log('parse error:', e.message);
    console.log(b.bodyPreview.slice(0, 4000));
  }
  console.log();
}
