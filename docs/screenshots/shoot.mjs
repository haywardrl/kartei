// Screenshots of the room, for the README and posts. Drives a headless
// Chromium over the DevTools protocol: open each URL, wait for the room to
// settle, capture. Needs the demo server on port 8765 with token devtoken:
//
//   go run ./cmd/kartei --demo --serve --no-open --port 8765 --token devtoken
//   node docs/screenshots/shoot.mjs docs/screenshots            # the standard set
//   node docs/screenshots/shoot.mjs out '[["night","hour=22",1160,620]]'
//
// CHROME can point at any Chromium binary; the default is Playwright's cache.
import { spawn } from 'node:child_process';
import { writeFileSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
const CH = process.env.CHROME || process.env.HOME + '/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell';
const OUT = process.argv[2] || 'docs/screenshots';
const model = await (await fetch('http://127.0.0.1:8765/api/vault', { headers: { 'X-Kartei-Token': 'devtoken' } })).json();
const card = model.notes.find((n) => n.filed && n.links.length)?.id || model.notes[0].id, unfiled = model.unfiled[0];
const STANDARD = [['room', 'hour=11', 1160, 620], ['room-night', 'hour=22', 1160, 620], ['drawer', 'hour=11&scene=drawer:1'], ['card', `hour=11&scene=card:${card}`],
  ['file', `hour=11&scene=file:${unfiled}`], ['desk', 'hour=11&scene=desk'], ['board', 'hour=11&scene=board'], ['register', 'hour=11&scene=register']];
const shots = process.argv[3] ? JSON.parse(process.argv[3]) : STANDARD;
const chrome = spawn(CH, ['--headless', '--disable-gpu', '--no-sandbox', '--hide-scrollbars', '--window-size=1400,900', '--remote-debugging-port=9333', `--user-data-dir=${mkdtempSync(join(tmpdir(), 'kartei-shots-'))}`, 'about:blank'], { stdio: 'ignore' });
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function cdp() {
  for (let i = 0; i < 40; i++) { try { const l = await (await fetch('http://127.0.0.1:9333/json/list')).json(); if (l.length) return l[0].webSocketDebuggerUrl; } catch {} await sleep(250); }
  throw new Error('no chrome');
}
const ws = new WebSocket(await cdp());
await new Promise((r) => ws.onopen = r);
let seq = 0; const pending = new Map();
ws.onmessage = (e) => { const m = JSON.parse(e.data); if (m.id && pending.has(m.id)) { pending.get(m.id)(m.result); pending.delete(m.id); } };
const send = (method, params = {}) => new Promise((r) => { const id = ++seq; pending.set(id, r); ws.send(JSON.stringify({ id, method, params })); });
await send('Network.enable'); await send('Network.setCacheDisabled', { cacheDisabled: true }); // always the current ui/
for (const [name, query, W = 1400, H = 820] of shots) {
  await send('Emulation.setDeviceMetricsOverride', { width: W, height: H, deviceScaleFactor: 1, mobile: false });
  await send('Page.navigate', { url: 'http://127.0.0.1:8765/?' + query });
  await sleep(2500);
  const { data } = await send('Page.captureScreenshot', { format: 'png' });
  writeFileSync(`${OUT}/${name}.png`, Buffer.from(data, 'base64'));
  console.log('shot', name, data.length > 20000 ? 'ok' : 'SMALL?');
}
ws.close(); chrome.kill();
