// One front end, two transports. Under Wails the calls go straight to the
// bound Go Bridge; in a browser they go over HTTP with the launch token.
// app.js only ever sees these named methods.
const wails = () => window.go?.main?.Bridge;
const TOKEN = document.querySelector('meta[name="kartei-token"]')?.content || '';

async function http(method, path, body) {
  const headers = { 'X-Kartei-Token': TOKEN };
  if (body) headers['Content-Type'] = 'application/json';
  const res = await fetch(path, { method, headers, body: body ? JSON.stringify(body) : undefined });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) { const e = new Error(data.error || res.statusText); e.data = data; e.status = res.status; throw e; }
  return data;
}

// Wails rejects with a string. A conflict is encoded as "conflict:<json>".
async function bound(fn) {
  try { return await fn(); } catch (err) {
    const msg = typeof err === 'string' ? err : err?.message || String(err);
    if (msg.startsWith('conflict:')) { const e = new Error('the note changed on disk since it was opened'); e.status = 409; e.data = JSON.parse(msg.slice(9)); throw e; }
    const e = new Error(msg); e.status = 400; throw e;
  }
}

export const api = {
  get native() { return !!wails(); },
  model: () => wails() ? bound(() => wails().Model()) : http('GET', '/api/vault'),
  note: (id) => wails() ? bound(() => wails().Note(id)) : http('GET', '/api/note/' + id),
  newNote: (title, body, parent = '') => wails() ? bound(() => wails().NewNote(title, body, parent)) : http('POST', '/api/note', { title, body, parent }),
  updateNote: (id, { title, body, seen }, force = false) => wails() ? bound(() => wails().UpdateNote(id, title, body, seen || '', force)) : http('PUT', '/api/note/' + id + (force ? '?force=1' : ''), { title, body, seen }),
  deleteNote: (id) => wails() ? bound(() => wails().DeleteNote(id)) : http('DELETE', '/api/note/' + id),
  adopt: (id) => wails() ? bound(() => wails().Adopt(id)) : http('POST', `/api/note/${id}/adopt`),
  file: (id, { parent = '', after = '', box = '' }) => wails() ? bound(() => wails().File(id, parent, box, after)) : http('POST', `/api/note/${id}/file`, { parent, after, box }),
  deskMove: (id, x, y) => wails() ? bound(() => wails().DeskMove(id, x, y)) : http('PUT', '/api/desk/' + id, { x, y }),
  newBoard: (name) => wails() ? bound(() => wails().NewBoard(name)) : http('POST', '/api/board', { name }),
  renameBoard: (bid, name) => wails() ? bound(() => wails().RenameBoard(bid, name)) : http('PUT', '/api/board/' + bid, { name }),
  deleteBoard: (bid) => wails() ? bound(() => wails().DeleteBoard(bid)) : http('DELETE', '/api/board/' + bid),
  pin: (bid, id, x, y) => wails() ? bound(() => wails().Pin(bid, id, x, y)) : http('POST', `/api/board/${bid}/pin/${id}`, { x, y }),
  unpin: (bid, id) => wails() ? bound(() => wails().Unpin(bid, id)) : http('DELETE', `/api/board/${bid}/pin/${id}`),
  registerAdd: (term, id = '', note = '') => wails() ? bound(() => wails().RegisterAdd(term, id, note)) : http('POST', '/api/register', { term, id, note }),
  registerRemove: (term, id = '') => wails() ? bound(() => wails().RegisterRemove(term, id)) : http('DELETE', `/api/register?term=${encodeURIComponent(term)}&id=${encodeURIComponent(id)}`),
  setPrefs: (p) => wails() ? bound(() => wails().SetPrefs(p)) : http('PUT', '/api/prefs', p),
  search: (q) => wails() ? bound(() => wails().Search(q)) : http('GET', '/api/search?q=' + encodeURIComponent(q)),

  // subscribe(cb) delivers the changed IDs after any change, from any source.
  subscribe(cb) {
    if (wails()) { window.runtime.EventsOn('changed', (ids) => cb(ids || [])); return; }
    const es = new EventSource('/api/events?token=' + encodeURIComponent(TOKEN));
    es.addEventListener('changed', (e) => { const ev = JSON.parse(e.data); cb(ev.ids || []); });
    es.onerror = () => { /* the browser reconnects on its own */ };
  },
  onMenu(cb) { if (wails()) window.runtime.EventsOn('menu', (cmd) => cb(cmd)); },
};
