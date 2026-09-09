// Orchestration: loads the model, wires the room to the panel, and turns
// intent into API calls. Nothing here draws or parses notes.
import { Room, BOARD } from './room.js';
import { Panel, esc, mini, addrChip, renderBody } from './panel.js';
import { sound } from './sound.js';
import { DESK_SPACE, BOX_DRAWERS } from './art.js';
import { api } from './api.js';

class App {
  constructor() {
    this.model = null; this.byId = new Map(); this.detail = null; this.activeBoard = null; this.details = new Map();
    this.room = new Room(document.getElementById('room'), {
      onClick: (o) => this.roomClick(o),
      onHover: (o) => this.tooltip(o),
      onCamera: (x) => this.savePrefs({ camera: Math.round(x) }),
    });
    this.panel = new Panel(document.getElementById('panel'), this);
    this.overlay = document.getElementById('overlay');
    this.wireHud();
    this.wireKeys();
  }

  async load() {
    this.model = await api.model();
    this.byId = new Map(this.model.notes.map((n) => [n.id, n]));
    this.drawerOf = new Map();
    for (const d of this.model.drawers) for (const id of d.ids) this.drawerOf.set(id, d);
    if (!this.activeBoard) this.activeBoard = this.model.prefs?.activeBoard || this.model.boards[0]?.id || null;
    this.room.setModel(this.model, this.activeBoard);
    sound.enabled = !!this.model.prefs?.sound;
    document.getElementById('btn-sound').textContent = sound.enabled ? 'sound on' : 'sound off';
    const filed = this.model.notes.filter((n) => n.filed).length;
    const hud = document.getElementById('hud-status');
    hud.textContent = `${filed} in the box · ${this.model.unfiled.length} on the desk · ${this.model.root.split('/').pop()}`; hud.title = this.model.root;
    return this.model;
  }

  async refresh(changedIds = []) {
    await this.load();
    const v = this.panel.view;
    if (this.overlay.classList.contains('boardfull')) this.renderBoard(this.activeBoard);
    if (this.overlay.classList.contains('deskfull')) this.renderDeskView();
    if (!v) return;
    if (v.name === 'card' && (changedIds.includes(v.arg) || !changedIds.length)) { await this.fetchDetail(v.arg); }
    if (v.name === 'editor') return; // never clobber typing
    this.panel.refresh();
  }

  isDescendant(candidate, ancestor) {
    let cur = this.byId.get(candidate);
    for (let i = 0; cur && cur.parent && i < 200; i++) { if (cur.parent === ancestor) return true; cur = this.byId.get(cur.parent); }
    return false;
  }

  // --- room ---
  roomClick(o) {
    if (!o) return;
    switch (o.kind) {
      case 'slipbox': this.turnBox('main'); break;
      case 'drawer':
        if (this.panel.view?.name === 'file') { this.panel.show('file', { id: this.panel.view.arg.id, drawer: o.index }); this.openDrawerInRoom(o.index); }
        else this.openDrawer(o.index);
        break;
      case 'cork': this.openBoard(this.activeBoard); break;
      case 'pad': this.newCard(); break;
      case 'inbox': case 'desk': this.openDesk(); break;
      case 'litbox': this.turnBox('lit'); break;
      case 'contents': this.openDrawers(); break;
      case 'ledger': this.openRegister(); break;
      case 'lamp': this.toggleLamp(); break;
    }
  }
  tooltip(o) {
    const t = document.getElementById('tooltip');
    t.hidden = !o; if (o) t.textContent = o.label;
  }
  toggleLamp() { this.room.lampOn = !this.room.lampOn; sound.lamp(); this.savePrefs({ lamp: this.room.lampOn }); }

  // --- navigation ---
  closePanel() { this.panel.hide(); }
  // Click a slip box: with more than one box in the stack, show the next;
  // otherwise open the contents.
  turnBox(box) {
    const { page, pages } = this.room.nextPage(box);
    if (pages <= 1) { this.openDrawers(); return; }
    sound.drawer(); this.room.closeDrawer();
    this.tooltip(this.room.objects().find((o) => o.kind === (box === 'main' ? 'slipbox' : 'litbox')));
  }
  openDrawers() {
    this.room.highlight = 'slipbox'; this.room.panToObject('slipbox'); this.room.closeDrawer();
    this.panel.show('drawers');
  }
  openDrawerInRoom(i) { sound.drawer(); this.room.highlight = 'slipbox'; this.room.panToObject('slipbox'); this.room.openDrawerAnim(i); this.savePrefs({ lastDrawer: i }); }
  openDrawer(i) { this.openDrawerInRoom(i); this.panel.show('drawer', i); }
  openDesk() { this.renderDeskView(); }
  openRegister() { sound.paper(); this.room.panToObject('desk'); this.panel.show('register'); }
  async openCard(id) {
    if (!this.byId.has(id)) { this.toast('That card is not in the box any more.'); return; }
    const fromDrawer = this.panel.view?.name === 'drawer' || this.panel.view?.name === 'drawers';
    if (fromDrawer) { sound.riffle(); this.room.flyFromSlipbox(); } else sound.paper();
    this.closeOverlay();
    this.panel.show('card', id);
    await this.fetchDetail(id);
    if (this.panel.view?.name === 'card' && this.panel.view.arg === id) this.panel.refresh();
  }
  async fetchDetail(id) {
    try { this.detail = await api.note(id); this.details.set(id, this.detail); } catch (e) { this.detail = null; }
  }
  // Full text for a set of cards, so a surface can show whole cards.
  prefetch(ids) {
    return Promise.all(ids.filter((id) => this.byId.has(id) && !this.details.has(id)).map((id) => api.note(id).then((d) => this.details.set(id, d)).catch(() => {})));
  }
  // Cards on a surface (the desk, a board) drag with the pointer; a press
  // without movement is a click. onDrop gets the element and its final
  // left/top; onMove runs during the drag.
  dragCards(host, selector, maxX, maxY, { onDrop, onClick, onMove }) {
    let drag = null;
    host.addEventListener('pointerdown', (e) => {
      if (e.target.closest('button') || e.target.closest('a') || e.target.closest('[data-unpin]')) return;
      const card = e.target.closest(selector); if (!card) return;
      drag = { card, dx: e.clientX - card.offsetLeft, dy: e.clientY - card.offsetTop, moved: false, sx: e.clientX, sy: e.clientY };
      card.classList.add('dragging'); card.setPointerCapture(e.pointerId);
    });
    host.addEventListener('pointermove', (e) => {
      if (!drag) return;
      if (Math.abs(e.clientX - drag.sx) + Math.abs(e.clientY - drag.sy) > 3) drag.moved = true;
      drag.card.style.left = Math.max(0, Math.min(maxX, e.clientX - drag.dx)) + 'px';
      drag.card.style.top = Math.max(0, Math.min(maxY, e.clientY - drag.dy)) + 'px';
      onMove?.();
    });
    const up = () => {
      if (!drag) return;
      const { card, moved } = drag; drag = null;
      card.classList.remove('dragging');
      if (moved) onDrop(card, parseInt(card.style.left), parseInt(card.style.top)); else onClick(card);
    };
    host.addEventListener('pointerup', up);
    host.addEventListener('pointercancel', up);
  }

  // --- writing ---
  writeBehind(parent) { this.detail = null; this.panel.show('editor', { parent }); }
  newCard() { this.detail = null; this.room.panToObject('desk'); this.panel.show('editor', { parent: '' }); }
  async edit(id) { await this.fetchDetail(id); this.panel.show('editor', { id }); }
  async cancelEdit() {
    const { id, parent } = this.panel.editing || {};
    if (this.panel.dirty() && !(await this.confirmSheet('Discard this card?', 'Nothing has been saved.', 'Discard'))) return;
    if (id) this.openCard(id); else if (parent) this.openCard(parent); else this.closePanel();
  }
  async saveCard({ id, parent, title, body, seen }, force = false) {
    try {
      let n;
      if (id) n = await api.updateNote(id, { title, body, seen }, force);
      else n = await api.newNote(title, body, parent || '');
      sound.save();
      await this.load();
      if (!id) this.toast(parent ? `Filed behind ${esc(this.byId.get(parent)?.address || '')} as ${n.address}` : 'On the desk. File it when you know where it belongs.', 'ok');
      this.openCard(n.id);
    } catch (e) {
      if (e.status === 409 && e.data?.conflict) { this.conflict({ id, title, body, seen }, e.data.disk); return; }
      this.toast(e.message);
    }
  }
  // The file changed on disk while it was open here. Never overwrite silently.
  conflict(mine, disk) {
    this.sheet(`<div class="conflict"><h2>This card changed on disk</h2>
      <p>Someone, probably you in another editor, saved this file after the room opened it. Which version should stay?</p>
      <h3 style="font:600 11px var(--mono);color:var(--ink-2);text-transform:uppercase">On disk</h3><pre>${esc(disk.body)}</pre>
      <h3 style="font:600 11px var(--mono);color:var(--ink-2);text-transform:uppercase">Yours, unsaved</h3><pre>${esc(mine.body)}</pre>
      <div class="actions"><button class="primary" data-c="mine">Keep mine</button><button data-c="theirs">Keep the disk version</button><button data-c="both">Keep both</button></div></div>`);
    this.overlay.querySelector('[data-c="mine"]').onclick = () => { this.closeOverlay(); this.saveCard(mine, true); };
    this.overlay.querySelector('[data-c="theirs"]').onclick = async () => { this.closeOverlay(); await this.load(); this.openCard(mine.id); };
    this.overlay.querySelector('[data-c="both"]').onclick = async () => {
      this.closeOverlay();
      try { await api.newNote(mine.title + ' (my version)', mine.body, ''); await this.load(); this.openCard(mine.id); this.toast('Kept the disk version; yours is a new card on the desk', 'ok'); } catch (e) { this.toast(e.message); }
    };
  }
  async trash(id) {
    const n = this.byId.get(id);
    if (!(await this.confirmSheet(`Trash "${n.title}"?`, 'It goes to .kartei/trash, never deleted. Cards filed behind it move up one level.', 'Trash it'))) return;
    try { await api.deleteNote(id); sound.paper(); await this.load(); this.toast('Moved to .kartei/trash', 'ok'); n.parent ? this.openCard(n.parent) : this.openDrawers(); } catch (e) { this.toast(e.message); }
  }

  // --- filing: the decision ---
  // pick = true opens the drawer chooser (both boxes listed) instead of
  // jumping into the last drawer used.
  fileCard(id, pick = false) {
    const n = this.byId.get(id);
    let drawer = pick ? null : n.filed ? this.model.drawers.findIndex((d) => d.ids.includes(id)) : (this.model.prefs?.lastDrawer ?? null);
    if (drawer != null && !(this.model.drawers[drawer]?.ids.length)) drawer = null; // an empty drawer has nothing to file behind
    this.room.highlight = 'slipbox'; this.room.panToObject('slipbox');
    this.panel.show('file', { id, drawer: drawer != null && drawer >= 0 ? drawer : null });
    if (drawer != null && drawer >= 0) this.openDrawerInRoom(drawer); else this.room.closeDrawer();
  }
  async doFile(id, parent, after, box = '') {
    try {
      const n = await api.file(id, { parent: parent || '', after: after || '', box });
      sound.place(); this.room.flyFromSlipbox();
      await this.load();
      const p = parent ? this.byId.get(parent) : null;
      this.toast(p ? `Filed behind ${p.address} as <b>${n.address}</b>` : after ? `Filed as <b>${n.address}</b>` : `A new branch: <b>${n.address}</b>`, 'ok');
      this.openCard(id);
    } catch (e) { sound.nope(); this.toast(e.message); }
  }

  // --- register ---
  async registerAdd(id) {
    const terms = this.model.register.map((e) => e.term);
    const term = await this.ask({ title: 'Enter in the register', text: 'Under which term, in your own words?', placeholder: 'term', suggestions: terms, ok: 'Enter' });
    if (!term) return;
    try { await api.registerAdd(term, id); sound.paper(); await this.load(); this.panel.refresh(); } catch (e) { this.toast(e.message); }
  }
  async registerAddTerm(term) {
    try { await api.registerAdd(term); await this.load(); this.panel.refresh(); } catch (e) { this.toast(e.message); }
  }
  async registerRemove(term, id) {
    if (!id && !(await this.confirmSheet(`Remove "${term}" from the register?`, 'The cards stay where they are.', 'Remove'))) return;
    try { await api.registerRemove(term, id); await this.load(); this.panel.refresh(); } catch (e) { this.toast(e.message); }
  }

  // --- desk ---
  async deskMove(id, x, y) { try { await api.deskMove(id, x, y); sound.place(); await this.load(); } catch (e) { this.toast(e.message); } }

  // --- the desk, seen from above: full screen ---
  async renderDeskView() {
    const m = this.model, byId = this.byId;
    this.overlay.hidden = false; this.overlay.className = 'deskfull';
    await this.prefetch(m.desk.map((p) => p.id));
    const cap = m.settings.deskCapacity || 12;
    // desk-local coordinates live in DESK_SPACE; the big desk is 1100 x 520
    const sx = (1100 - 240) / DESK_SPACE.w, sy = (520 - 170) / DESK_SPACE.h;
    let onDesk = 0;
    const cardHtml = m.desk.map((p, i) => {
      const n = byId.get(p.id); if (!n) return '';
      const placed = p.x >= 0;
      if (!placed && onDesk >= cap) return '';
      onDesk++;
      const lx = placed ? p.x : (i * 23) % DESK_SPACE.w, ly = placed ? p.y : (i * 5) % DESK_SPACE.h;
      const d = this.details.get(n.id);
      const body = d ? renderBody(d, byId) : `<p>${esc(n.excerpt)}</p>`;
      return `<div class="desk-card" data-id="${n.id}" data-lx="${lx}" data-ly="${ly}" style="left:${Math.round(lx * sx)}px;top:${Math.round(ly * sy)}px;transform:rotate(${((i * 7) % 5) - 2}deg)">
        <div class="card-head"><span class="addr unfiled">unfiled</span><span class="t">${esc(n.title)}</span></div>
        <div class="card-body">${body}</div>
        <div class="desk-actions"><button data-file="${n.id}">file</button><button data-edit="${n.id}">edit</button></div></div>`;
    }).join('');
    const pile = m.desk.length - onDesk;
    const redis = m.rediscover && m.prefs?.rediscoverAway !== m.rediscover ? byId.get(m.rediscover) : null;
    this.overlay.innerHTML = `<div class="deskwrap">
      <div class="boardbar"><h2>The desk</h2><span class="hint">${m.desk.length ? `${m.desk.length} card${m.desk.length === 1 ? '' : 's'} waiting to be filed · drag to arrange · click a card to open it` : 'Clear. Everything is in the box.'}</span><span class="spacer"></span><button data-desk-new>new card</button><button data-desk-close>close</button></div>
      <div class="deskview"><div class="desk big" id="deskbig">
        ${cardHtml}
        <div class="desk-object pad" data-desk-new title="A pad of blank cards"><span>blank cards</span></div>
        <div class="desk-object contents" data-desk-contents title="Contents: what is in each drawer"><span>contents</span></div>
        <div class="desk-object ledger" data-desk-register title="The register"><span>register</span></div>
        ${redis ? `<div class="desk-object slipped" data-open="${redis.id}" title="Slipped out of the box today"><span class="tab"></span><b>${esc(redis.title)}</b><i>slipped out of the box today</i><button data-redis-away title="Back in the box until tomorrow">put it away</button></div>` : ''}
        ${pile ? `<div class="desk-object pile" data-desk-pile title="${pile} more cards in the pile"><span>+${pile} in the pile</span></div>` : ''}
      </div></div></div>`;
    this.wireDeskView(sx, sy);
  }
  wireDeskView(sx, sy) {
    const o = this.overlay, desk = o.querySelector('#deskbig');
    o.onclick = (e) => {
      const t = e.target;
      if (t.closest('[data-desk-close]')) { this.closeOverlay(); return; }
      if (t.closest('[data-desk-new]')) { this.closeOverlay(); this.newCard(); return; }
      if (t.closest('[data-desk-contents]')) { this.closeOverlay(); this.openDrawers(); return; }
      if (t.closest('[data-desk-register]')) { this.closeOverlay(); this.openRegister(); return; }
      if (t.closest('[data-desk-pile]')) { this.closeOverlay(); this.panel.show('desk'); return; }
      const f = t.closest('[data-file]'); if (f) { this.closeOverlay(); this.fileCard(f.dataset.file); return; }
      const ed = t.closest('[data-edit]'); if (ed) { this.closeOverlay(); this.edit(ed.dataset.edit); return; }
      if (t.closest('[data-redis-away]')) { this.savePrefs({ rediscoverAway: this.model.rediscover }); this.room.setModel(this.model, this.activeBoard); this.renderDeskView(); return; }
      const op = t.closest('[data-open]'); if (op && !t.closest('.desk-card')) { this.closeOverlay(); this.openCard(op.dataset.open); return; }
      if (e.target === o) this.closeOverlay();
    };
    this.dragCards(desk, '.desk-card', 1100 - 240, 520 - 170, {
      onDrop: (card, x, y) => this.deskMove(card.dataset.id, Math.round(x / sx), Math.round(y / sy)),
      onClick: (card) => { this.closeOverlay(); this.openCard(card.dataset.id); },
    });
    desk.addEventListener('click', (e) => { const a = e.target.closest('a[data-open]'); if (a) { e.stopPropagation(); this.closeOverlay(); this.openCard(a.dataset.open); } });
  }

  // --- boards: full screen ---
  openBoard(bid) {
    this.activeBoard = bid || this.model.boards[0]?.id || null;
    this.room.highlight = 'cork'; this.room.setModel(this.model, this.activeBoard);
    this.savePrefs({ activeBoard: this.activeBoard });
    this.renderBoard(this.activeBoard);
  }
  async renderBoard(bid) {
    const m = this.model, byId = this.byId;
    const b = m.boards.find((x) => x.id === bid) || m.boards[0];
    this.overlay.hidden = false; this.overlay.className = 'boardfull';
    const tabs = m.boards.map((x) => `<button class="${x.id === b?.id ? 'on' : ''}" data-board="${x.id}">${esc(x.name)}</button>`).join('') + '<button data-board-new>+ new board</button>';
    const bar = `<div class="boardbar"><h2>Corkboard</h2>${tabs}<span class="spacer"></span>${b ? `<button data-board-rename="${b.id}">rename</button><button data-board-delete="${b.id}">delete</button>` : ''}<span class="hint">drag to arrange · click a card to open · red thread = the cards link to each other</span><button data-board-close>close</button></div>`;
    if (!b) { this.overlay.innerHTML = `<div class="boardwrap">${bar}<div class="boardview"><p class="empty" style="color:#fbe9c8">No boards yet. A board is a working set: pin cards from the box without moving them.</p></div></div>`; this.wireBoardBar(); return; }
    await this.prefetch(b.cards.map((c) => c.id));
    const cards = b.cards.map((c) => {
      const n = byId.get(c.id); if (!n) return '';
      const d = this.details.get(c.id);
      const body = d ? renderBody(d, byId) : `<p>${esc(n.excerpt)}</p>`;
      return `<div class="pin-card${n.filed ? '' : ' unfiled'}" data-id="${n.id}" style="left:${Math.min(c.x, BOARD.w - 220)}px;top:${Math.min(c.y, BOARD.h - 80)}px"><span class="pin"></span><span class="x" data-unpin="${n.id}" title="Unpin">×</span><div class="t">${esc(n.title)}</div><div class="body">${body}</div>${addrChip(n)}</div>`;
    }).join('');
    this.overlay.innerHTML = `<div class="boardwrap">${bar}<div class="boardview"><div class="board big" id="board" data-bid="${b.id}"><svg class="threads"></svg>${cards || '<div class="board-empty">Open a card and press p to pin it here.</div>'}</div></div></div>`;
    this.wireBoardBar(); this.wireBoard(); this.drawThreads();
  }
  drawThreads() {
    const board = document.getElementById('board'); if (!board) return;
    const svg = board.querySelector('svg.threads');
    const cards = [...board.querySelectorAll('.pin-card')];
    const pos = new Map(cards.map((c) => [c.dataset.id, { x: c.offsetLeft + c.offsetWidth / 2, y: c.offsetTop + 8 }]));
    const lines = [];
    const seen = new Set();
    for (const c of cards) {
      const n = this.byId.get(c.dataset.id); if (!n) continue;
      for (const t of n.links || []) {
        const key = [n.id, t].sort().join('|');
        if (pos.has(t) && !seen.has(key)) { seen.add(key); const a = pos.get(n.id), b = pos.get(t); lines.push(`<line x1="${a.x}" y1="${a.y}" x2="${b.x}" y2="${b.y}"></line>`); }
      }
    }
    svg.innerHTML = lines.join('');
  }
  wireBoardBar() {
    const o = this.overlay;
    o.onclick = (e) => {
      const t = e.target.closest('button'); if (!t) return;
      if (t.dataset.board) this.openBoard(t.dataset.board);
      else if (t.hasAttribute('data-board-new')) this.newBoard();
      else if (t.dataset.boardRename) this.renameBoard(t.dataset.boardRename);
      else if (t.dataset.boardDelete) this.deleteBoard(t.dataset.boardDelete);
      else if (t.hasAttribute('data-board-close')) this.closeOverlay();
    };
  }
  wireBoard() {
    const board = document.getElementById('board'); if (!board) return;
    this.dragCards(board, '.pin-card', BOARD.w - 220, BOARD.h - 80, {
      onDrop: (card, x, y) => this.movePin(board.dataset.bid, card.dataset.id, x, y),
      onClick: (card) => this.openCard(card.dataset.id),
      onMove: () => this.drawThreads(),
    });
    board.addEventListener('click', (e) => {
      const u = e.target.closest('[data-unpin]'); if (u) { e.stopPropagation(); this.unpin(board.dataset.bid, u.dataset.unpin); return; }
      const a = e.target.closest('a[data-open]'); if (a) { e.stopPropagation(); this.openCard(a.dataset.open); }
    });
  }
  async newBoard() {
    const name = await this.ask({ title: 'New board', text: 'A project, an essay, a question.', placeholder: 'name', ok: 'Create' }); if (!name) return;
    try { const b = await api.newBoard(name); sound.pin(); await this.load(); this.openBoard(b.id); } catch (e) { this.toast(e.message); }
  }
  async renameBoard(bid) {
    const b = this.model.boards.find((x) => x.id === bid); const name = await this.ask({ title: 'Rename board', value: b?.name || '', ok: 'Rename' }); if (!name) return;
    try { await api.renameBoard(bid, name); await this.load(); this.renderBoard(bid); } catch (e) { this.toast(e.message); }
  }
  async deleteBoard(bid) {
    if (!(await this.confirmSheet('Delete this board?', 'The cards stay in their drawers.', 'Delete'))) return;
    try { await api.deleteBoard(bid); this.activeBoard = null; await this.load(); this.openBoard(this.model.boards[0]?.id); } catch (e) { this.toast(e.message); }
  }
  async pin(id) {
    if (!this.model.boards.length) { await this.newBoard(); if (!this.model.boards.length) return; }
    let bid = this.activeBoard || this.model.boards[0].id;
    if (this.model.boards.length > 1) {
      bid = await this.choose('Pin to which board?', this.model.boards.map((b) => ({ label: b.name, value: b.id })));
      if (!bid) return;
    }
    const b = this.model.boards.find((x) => x.id === bid);
    const n = b.cards.length;
    const x = 40 + (n % 4) * 280 + (n * 13) % 30, y = 40 + Math.floor(n / 4) * 240 % (BOARD.h - 120);
    try { await api.pin(bid, id, x, y); sound.pin(); await this.load(); this.activeBoard = bid; this.room.setModel(this.model, bid); this.panel.refresh(); }
    catch (e) { this.toast(e.message); }
  }
  async movePin(bid, id, x, y) { try { await api.pin(bid, id, x, y); sound.pin(); await this.load(); this.room.setModel(this.model, bid); } catch (e) { this.toast(e.message); } }
  async unpin(bid, id) { try { await api.unpin(bid, id); sound.paper(); await this.load(); this.room.setModel(this.model, bid); this.renderBoard(bid); } catch (e) { this.toast(e.message); } }

  // --- dialogs: in-page, because the native web view has no prompt() or confirm() ---
  ask({ title, text = '', value = '', placeholder = '', suggestions = [], ok = 'OK' }) {
    return new Promise((resolve) => {
      const list = suggestions.length ? `<datalist id="ask-list">${suggestions.map((s) => `<option value="${esc(s)}">`).join('')}</datalist>` : '';
      this.sheet(`<h2>${esc(title)}</h2>${text ? `<p>${text}</p>` : ''}<input id="ask-input" list="ask-list" value="${esc(value)}" placeholder="${esc(placeholder)}" autocomplete="off">${list}<div class="actions" style="margin-top:12px"><button class="primary" data-ask="ok">${esc(ok)}</button><button data-ask="cancel">Cancel</button></div>`);
      const input = this.overlay.querySelector('#ask-input');
      const done = (v) => { this.closeOverlay(); resolve(v); };
      this.overlay.querySelector('[data-ask="ok"]').onclick = () => done(input.value.trim() || null);
      this.overlay.querySelector('[data-ask="cancel"]').onclick = () => done(null);
      input.onkeydown = (e) => { if (e.key === 'Enter') done(input.value.trim() || null); if (e.key === 'Escape') done(null); };
      this.overlay.onclick = (e) => { if (e.target === this.overlay) done(null); };
      input.focus(); input.select();
    });
  }
  confirmSheet(title, text = '', ok = 'Yes') {
    return new Promise((resolve) => {
      this.sheet(`<h2>${esc(title)}</h2>${text ? `<p>${text}</p>` : ''}<div class="actions" style="margin-top:12px"><button class="primary" data-ask="ok">${esc(ok)}</button><button data-ask="cancel">Cancel</button></div>`);
      const done = (v) => { this.closeOverlay(); resolve(v); };
      this.overlay.querySelector('[data-ask="ok"]').onclick = () => done(true);
      this.overlay.querySelector('[data-ask="cancel"]').onclick = () => done(false);
      this.overlay.onclick = (e) => { if (e.target === this.overlay) done(false); };
      this.overlay.querySelector('[data-ask="ok"]').focus();
    });
  }
  choose(title, options) { // options: [{label, value}]
    return new Promise((resolve) => {
      this.sheet(`<h2>${esc(title)}</h2><div class="actions" style="flex-direction:column;align-items:stretch">${options.map((o, i) => `<button data-choose="${i}">${esc(o.label)}</button>`).join('')}<button data-ask="cancel">Cancel</button></div>`);
      const done = (v) => { this.closeOverlay(); resolve(v); };
      this.overlay.querySelectorAll('[data-choose]').forEach((b) => { b.onclick = () => done(options[+b.dataset.choose].value); });
      this.overlay.querySelector('[data-ask="cancel"]').onclick = () => done(null);
      this.overlay.onclick = (e) => { if (e.target === this.overlay) done(null); };
    });
  }

  // --- overlays ---
  sheet(html) { this.overlay.hidden = false; this.overlay.className = ''; this.overlay.innerHTML = `<div class="sheet">${html}</div>`; this.overlay.onclick = (e) => { if (e.target === this.overlay) this.closeOverlay(); }; }
  closeOverlay() { this.overlay.hidden = true; this.overlay.className = ''; this.overlay.innerHTML = ''; this.overlay.onclick = null; if (this.room.highlight === 'cork') this.room.highlight = null; }
  search() {
    this.sheet(`<input id="q" placeholder="Search titles, then bodies…" autocomplete="off"><div class="results" id="results"></div><div class="hint">↑↓ move · ⏎ open · Esc close</div>`);
    const q = this.overlay.querySelector('#q'), r = this.overlay.querySelector('#results');
    let hits = [], sel = 0, timer = null;
    const paint = () => { r.innerHTML = hits.map((h, i) => mini(h).replace('class="mini', `class="mini${i === sel ? ' sel' : ''}`).replace('<div ', i === sel ? '<div style="border-color:var(--accent-2);transform:translateX(4px)" ' : '<div ')).join('') || (q.value ? '<p class="empty">Nothing matches.</p>' : ''); };
    q.oninput = () => { clearTimeout(timer); timer = setTimeout(async () => { hits = q.value.trim() ? await api.search(q.value) : []; sel = 0; paint(); }, 80); };
    q.onkeydown = (e) => {
      if (e.key === 'ArrowDown') { sel = Math.min(sel + 1, hits.length - 1); paint(); e.preventDefault(); }
      else if (e.key === 'ArrowUp') { sel = Math.max(sel - 1, 0); paint(); e.preventDefault(); }
      else if (e.key === 'Enter' && hits[sel]) { this.openCard(hits[sel].id); }
    };
    r.onclick = (e) => { const m = e.target.closest('[data-open]'); if (m) this.openCard(m.dataset.open); };
    q.focus();
  }
  help() {
    this.sheet(`<h2>The study</h2>
      <p><b>The desk</b> is the inbox: a card you write from the pad, or in any other editor, lies there with no address. <b>Filing</b> puts it in the box: choose the card it sits <i>behind</i> (a branch off it) or <i>after</i> (next in its line). That decision gives it an address like 2b1 and a drawer, and clears it from the desk. <b>Drawers</b> fill in address order, a whole branch at a time; click one to open it. <b>Links</b> in the text say what a card mentions, filing says where it grew from. The <b>ledger</b> is your register: a term, then the drawers where it lives. The <b>corkboard</b> pins references for a project.</p>
      <table>
        <tr><td><kbd>/</kbd></td><td>search</td><td><kbd>n</kbd></td><td>new card</td></tr>
        <tr><td><kbd>s</kbd></td><td>slip box</td><td><kbd>b</kbd></td><td>corkboard</td></tr>
        <tr><td><kbd>i</kbd></td><td>the desk</td><td><kbd>r</kbd></td><td>register</td></tr>
        <tr><td><kbd>f</kbd></td><td>file / refile the open card</td><td><kbd>w</kbd></td><td>write behind it</td></tr>
        <tr><td><kbd>p</kbd></td><td>pin to a board</td><td><kbd>e</kbd></td><td>edit card</td></tr>
        <tr><td><kbd>l</kbd></td><td>lamp</td><td><kbd>Esc</kbd></td><td>close</td></tr>
      </table>
      <p class="hint">Notes are plain markdown in ${esc(this.model?.root || 'your vault')}. Edit them in any editor; the room updates as you save. Structure lives in .kartei/state.json.</p>`);
  }
  toast(msg, kind = '') {
    const t = document.createElement('div'); t.className = 'toast ' + kind; t.innerHTML = msg;
    const host = document.getElementById('toasts');
    while (host.children.length >= 3) host.firstChild.remove();
    host.appendChild(t);
    setTimeout(() => t.remove(), 2800);
  }

  // --- prefs ---
  savePrefs(p) {
    this.model.prefs = { ...(this.model.prefs || {}), ...p };
    clearTimeout(this.prefTimer);
    this.prefTimer = setTimeout(() => api.setPrefs(p).catch(() => {}), 400);
  }

  wireHud() {
    document.getElementById('btn-sound').onclick = () => { sound.enabled = !sound.enabled; document.getElementById('btn-sound').textContent = sound.enabled ? 'sound on' : 'sound off'; this.savePrefs({ sound: sound.enabled }); if (sound.enabled) sound.paper(); };
    document.getElementById('btn-lamp').onclick = () => this.toggleLamp();
    document.getElementById('btn-search').onclick = () => this.search();
    document.getElementById('btn-new').onclick = () => this.newCard();
    document.getElementById('btn-help').onclick = () => this.help();
  }
  wireKeys() {
    document.addEventListener('keydown', (e) => {
      const typing = ['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName);
      if (e.key === 'Escape') { if (!this.overlay.hidden) { this.closeOverlay(); return; } if (typing) return; if (this.panel.isOpen()) this.closePanel(); return; }
      if (typing || e.metaKey || e.ctrlKey || e.altKey) return;
      const v = this.panel.view;
      const card = v?.name === 'card' ? v.arg : null;
      const n = card ? this.byId.get(card) : null;
      // Stop the key landing in whatever input the shortcut focuses (n opened
      // the editor and then typed "n" into the title).
      if ('/?nsbirlfwpe'.includes(e.key) && e.key.length === 1) e.preventDefault();
      switch (e.key) {
        case '/': this.search(); break;
        case '?': this.help(); break;
        case 'n': this.newCard(); break;
        case 's': this.openDrawers(); break;
        case 'b': this.openBoard(this.activeBoard); break;
        case 'i': this.openDesk(); break;
        case 'r': if (card && n && !n.guest) this.registerAdd(card); else this.openRegister(); break;
        case 'l': this.toggleLamp(); break;
        case 'f': if (n && !n.guest && !n.damaged) this.fileCard(card); break;
        case 'w': if (n && n.filed) this.writeBehind(card); break;
        case 'p': if (card) this.pin(card); break;
        case 'e': if (n && !n.guest && !n.damaged) this.edit(card); break;
      }
    });
  }

  live() {
    api.subscribe((ids) => { for (const id of ids) this.details.delete(id); this.refresh(ids); });
    api.onMenu((cmd) => ({
      'new-card': () => this.newCard(), contents: () => this.openDrawers(), desk: () => this.openDesk(), board: () => this.openBoard(this.activeBoard),
      register: () => this.openRegister(), lamp: () => this.toggleLamp(), search: () => this.search(), help: () => this.help(),
      sound: () => document.getElementById('btn-sound').click(), 'vault-changed': () => { this.closePanel(); this.closeOverlay(); this.activeBoard = null; this.load(); },
    }[cmd] || (() => {}))());
  }
}

const app = new App();
window.kartei = app; // handy for poking at it from the console
app.load().then(() => {
  app.live();
  // Development hooks for screenshots and checks: ?hour=21 fixes the clock,
  // ?scene=desk|board|drawers|drawer:2|card:<id>|file:<id>|register opens a view.
  const q = new URLSearchParams(location.search);
  if (q.has('hour')) app.room.hourOverride = parseFloat(q.get('hour'));
  const [scene, arg] = (q.get('scene') || '').split(':');
  ({ desk: () => app.openDesk(), board: () => app.openBoard(app.activeBoard), drawers: () => app.openDrawers(),
     drawer: () => app.openDrawer(+arg), card: () => app.openCard(arg), file: () => app.fileCard(arg, true), register: () => app.openRegister() }[scene] || (() => {}))();
});
