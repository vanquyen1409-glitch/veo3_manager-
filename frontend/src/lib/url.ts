// Encode an absolute path to /localfile/<base64url> for use in <video src>.
// Mirror of internal/server.LocalFileHandler decoding.
export function localFileURL(absPath: string): string {
  const utf8 = new TextEncoder().encode(absPath);
  let bin = '';
  utf8.forEach((b) => {
    bin += String.fromCharCode(b);
  });
  const b64 = btoa(bin)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
  return `/localfile/${b64}`;
}
