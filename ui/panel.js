// The paper panel: everything that needs real text. Drawers, the pulled card
// with its stack, the editor, filing, the inbox and the register. Renders
// from app state and calls back into the app for every change.
export const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

function fmtDate(iso) {
  const d = new Date(iso);
  return isNaN(d) ? '' : d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

// Body text to HTML: paragraphs, inline code, and wikilinks resolved through
// the link list the engine already computed.
export function renderBody(detail, byId) {
  const resolved = new Map(detail.links.map((l) => [l.raw, l]));
  const inline = (text) => esc(text)
    .replace(/`([^`]+)`/g, (_, c) => `<code>${c}</code>`)
    .replace(/\[\[([^\]|]+)(?:\|([^\]]+))?\]\]/g, (raw, target, alias) => {
      const l = resolved.get(raw.replace(/&quot;/g, '"')) || [...resolved.values()].find((x) => x.target === target);
      const note = l?.resolved ? byId.get(l.resolved) : null;
      if (note) return `<a class="wl" data-open="${note.id}">${esc(alias || note.title)}</a>`;
      return `<a class="wl broken" title="does not resolve">${esc(alias || target)}</a>`;
    });
  return detail.body.split(/\n\s*\n/).filter((p) => p.trim()).map((p) => `<p>${inline(p.trim()).replace(/\n/g, '<br>')}</p>`).join('');
}

const thick = (n) => n === 0 ? '' : `<span class="thick" title="${n} behind">${'▮'.repeat(Math.min(n, 6))} ${n}</span>`;

export function addrChip(n) {
  if (n.guest) return '<span class="addr soft">guest</span>';
  if (!n.filed) return '<span class="addr unfiled">unfiled</span>';
  return `<span class="addr soft">${esc(n.address)}</span>`;
}

// "Drawer 3", "Literature drawer 1", "Unsorted": how a drawer is named in text.
export function drawerName(d, m) {
  if (!d) return '';
  if (d.unsorted) return 'Unsorted';
  return `${d.box !== m.boxes[0].id ? 'Literature drawer' : 'Drawer'} ${d.number}`;
}

export function mini(n, extra = '') {
  if (!n) return '';
  return `<div class="mini${n.filed || n.guest ? '' : ' unfiled'}" data-open="${n.id}">${addrChip(n)}<span class="t">${esc(n.title)}${n.damaged ? ' <span class="tag">damaged</span>' : ''}<div class="ex">${esc(n.excerpt)}</div></span>${extra}${thick(n.children?.length || 0)}</div>`;
}

export class Panel {
  constructor(el, app) {
    this.el = el; this.app = app; this.view = null; this.history = [];
    el.addEventListener('click', (e) => this.onClick(e));
  }

  hide() { this.el.hidden = true; this.view = null; this.history = []; this.app.room.highlight = null; this.app.room.closeDrawer(); this.app.room.resize(); }
  isOpen() { return !this.el.hidden; }

  show(view, arg) {
    // Keep a trail so the back button can retrace it. Editors and the filing
    // flow are steps, not places, so they are not kept.
    if (this.view && !this.goingBack && !['editor', 'file'].includes(this.view.name) && !(this.view.name === view && this.view.arg === arg)) this.history.push(this.view);
    if (this.history.length > 30) this.history.shift();
    this.goingBack = false;
    this.view = { name: view, arg };
    const wasHidden = this.el.hidden;
    this.el.hidden = false;
    this.render();
    this.el.scrollTop = 0;
    if (wasHidden) this.app.room.resize();
  }
  refresh() { if (this.view) this.render(); }

  render() {
    const { name, arg } = this.view;
    const r = { drawers: this.drawers, drawer: this.drawer, card: this.card, editor: this.editor, file: this.file, desk: this.desk, register: this.register }[name];
    this.el.innerHTML = r.call(this, arg);
    if (name === 'editor') this.wireEditor(arg);
    if (name === 'file') this.wireFiling(arg);
    if (name === 'register') this.wireRegister();
  }

  close() { return `<span class="panel-btns">${this.history.length ? '<button data-act="back" title="Back">‹ back</button>' : ''}<button data-act="close" title="Close (Esc)">close</button></span>`; }
  back() {
    const v = this.history.pop(); if (!v) return;
    this.goingBack = true;
    const a = this.app;
    ({ drawers: () => a.openDrawers(), drawer: () => a.openDrawer(v.arg), card: () => a.openCard(v.arg), desk: () => this.show('desk'), register: () => a.openRegister() }[v.name] || (() => { this.goingBack = false; }))();
  }
  crumbs(parts) { return `<div class="crumbs">${parts.map((p) => p.act ? `<a data-act="${p.act}" data-arg="${p.arg ?? ''}">${esc(p.text)}</a>` : `<span>${esc(p.text)}</span>`).join('<span>›</span>')}</div>`; }

  // --- views ---
  drawers() {
    const m = this.app.model;
    const total = m.notes.filter((n) => n.filed).length;
    const last = m.prefs?.lastDrawer;
    return `<div class="panel-top"><h2>Contents</h2>${this.close()}</div>
      <div class="meta">${total} cards filed · ${m.drawers.length} drawer${m.drawers.length === 1 ? '' : 's'} · ${m.settings.drawerSize} cards per drawer</div>
      ${m.incomplete ? '<div class="warn">This folder looks incomplete (many placed cards are missing). Nothing will be rewritten until it looks whole again.</div>' : ''}
      ${m.unfiled.length ? `<div class="banner">${m.unfiled.length} card${m.unfiled.length === 1 ? ' is' : 's are'} on the desk, not yet in the box. <a class="wl" data-act="desk">See them</a>.</div>` : ''}
      ${m.boxes.map((b) => { const ds = m.drawers.filter((d) => d.box === b.id && !d.unsorted); return `<div class="boxhead">${esc(b.name)}${b.prefix ? ` · addresses ${esc(b.prefix)}1, ${esc(b.prefix)}1a…` : ''}</div>` + (ds.map((d) => `<div class="drawer-front" data-drawer="${d.index}"><span class="label">${esc(d.label)}</span><span class="count">${d.ids.length} card${d.ids.length === 1 ? '' : 's'}${last === d.index ? ' · last opened' : ''}</span></div>`).join('') || '<p class="empty">Empty. File a card as a new branch here to start it.</p>'); }).join('')}
      ${m.drawers.filter((d) => d.unsorted).map((d) => `<div class="boxhead">Unsorted</div><div class="drawer-front unsorted" data-drawer="${d.index}"><span class="label">${esc(d.label)}</span><span class="count">${d.ids.length} card${d.ids.length === 1 ? '' : 's'}</span></div>`).join('')}
      <p class="empty" style="margin-top:14px">${m.settings.drawerRule === 'address-range' ? 'Drawers are chunks of one continuous sequence, not topics.' : 'One drawer per branch. A branch that outgrows its drawer spills into the next.'}</p>`;
  }

  drawer(i) {
    const m = this.app.model;
    const d = m.drawers[i];
    if (!d) return this.drawers();
    const byId = this.app.byId;
    return `${this.crumbs([{ text: 'Slip box', act: 'drawers' }, { text: drawerName(d, m) }])}
      <div class="panel-top"><h2>${esc(d.unsorted ? 'Unsorted' : d.label)}</h2>${this.close()}</div>
      <div class="meta">${d.ids.length} card${d.ids.length === 1 ? '' : 's'}${d.first ? ` · ${esc(d.first)} to ${esc(d.last)}` : ''}${d.unsorted ? ' · guests and damaged notes live here until adopted' : ''}</div>
      <div class="fan">${d.ids.map((id) => mini(byId.get(id))).join('') || '<p class="empty">Empty drawer.</p>'}</div>`;
  }

  card(id) {
    const m = this.app.model, byId = this.app.byId;
    const n = byId.get(id), detail = this.app.detail;
    if (!n) return `<div class="panel-top"><h2>Card not found</h2>${this.close()}</div><p class="empty">It may have been moved or deleted outside the room.</p>`;
    const parent = n.parent ? byId.get(n.parent) : null;
    const drawerIndex = m.drawers.findIndex((d) => d.ids.includes(id));
    const kids = n.children.map((k) => byId.get(k)).filter(Boolean);
    const drawer = m.drawers[drawerIndex];
    const head = `<div class="card-head">${addrChip(n)}<span class="t">${esc(n.title)}</span></div>`;
    const body = detail && detail.id === id ? (n.damaged ? `<div class="card-paper damaged">${head}<div class="card-body">${esc(detail.body)}</div></div>` : `<div class="card-paper">${head}<div class="card-body">${renderBody(detail, byId) || '<p class="empty">Blank card.</p>'}</div></div>`) : `<div class="card-paper">${head}<div class="card-body"><p class="empty">Loading…</p></div></div>`;
    const xrefs = detail && detail.id === id ? detail.crossrefs.map((x) => byId.get(x)).filter(Boolean) : [];
    const pinnedOn = m.boards.filter((b) => b.cards.some((c) => c.id === id));
    const terms = m.register.filter((e) => e.targets.includes(id));
    const canFile = !n.guest && !n.damaged;
    return `${this.crumbs([{ text: 'Slip box', act: 'drawers' }, ...(drawer ? [{ text: drawerName(drawer, m), act: 'drawer', arg: drawerIndex }] : []), ...(parent ? [{ text: parent.address + ' ' + parent.title, act: 'open', arg: parent.id }] : []), { text: n.filed ? n.address : n.guest ? 'guest' : 'unfiled' }])}
      <div class="panel-top"><span class="meta" style="margin:0">${fmtDate(n.created)} · <span title="${esc(n.path)}">${esc(n.path)}</span></span>${this.close()}</div>
      ${n.damaged ? '<div class="warn">This note\'s frontmatter could not be read. It is shown as-is and will never be rewritten by the room.</div>' : ''}
      ${n.guest ? '<div class="warn">A guest: this file was not written by the room. It can be read and linked, but not filed or placed until adopted.</div>' : ''}
      ${canFile && !n.filed ? '<div class="banner"><b>On the desk, not in the box yet.</b> It has no address until you decide what it sits behind. Filing is the decision.</div>' : ''}
      ${body}
      <div class="actions">
        ${canFile && !n.filed ? `<button class="primary" data-act="file" data-arg="${id}">File this card<kbd>f</kbd></button>` : ''}
        ${canFile && n.filed ? `<button class="primary" data-act="write-behind" data-arg="${id}">Write behind this<kbd>w</kbd></button>` : ''}
        ${n.guest ? '' : `<button data-act="pin" data-arg="${id}">Pin to board<kbd>p</kbd></button>`}
        ${canFile ? `<button data-act="register-add" data-arg="${id}">Register<kbd>r</kbd></button>` : ''}
        ${canFile ? `<button data-act="edit" data-arg="${id}">Edit<kbd>e</kbd></button>` : ''}
        ${canFile && n.filed ? `<button data-act="file" data-arg="${id}">Refile<kbd>f</kbd></button>` : ''}
        <button class="danger" data-act="trash" data-arg="${id}">Trash</button>
      </div>
      ${pinnedOn.length ? `<div class="meta">Pinned on ${pinnedOn.map((b) => `<a class="wl" data-act="board" data-arg="${b.id}">${esc(b.name)}</a>`).join(', ')}</div>` : ''}
      ${terms.length ? `<div class="meta">In the register under ${terms.map((t) => `<a class="wl" data-act="register">${esc(t.term)}</a>`).join(', ')}${drawer && !drawer.unsorted ? ` · ${drawerName(drawer, m).toLowerCase()}` : ''}</div>` : ''}
      ${n.filed ? `<h3>Behind this card · ${kids.length}</h3>
      <div class="stack">${kids.slice(0, 5).map((k) => mini(k)).join('')}${kids.length > 5 ? `<div class="more" data-act="drawer" data-arg="${drawerIndex}">+ ${kids.length - 5} more behind</div>` : ''}${kids.length === 0 ? '<p class="empty">Nothing yet. Write the next thought behind this one.</p>' : ''}</div>` : ''}
      <h3>Cross-references · ${xrefs.length}</h3>
      <div class="xref">${xrefs.map((x) => mini(x)).join('') || '<p class="empty">No links in or out beyond this branch.</p>'}</div>
      ${parent ? `<h3>Filed behind</h3>${mini(parent)}` : n.filed ? '<h3>A root</h3><p class="empty">This card starts a branch.</p>' : ''}`;
  }

  editor({ id, parent }) {
    const byId = this.app.byId;
    const editing = id ? byId.get(id) : null;
    const p = parent ? byId.get(parent) : editing?.parent ? byId.get(editing.parent) : null;
    const detail = this.app.detail;
    const body = editing && detail && detail.id === id ? detail.body : '';
    return `<div class="panel-top"><h2>${editing ? 'Edit card' : 'New card'}</h2>${this.close()}</div>
      <div class="editor">
        <div class="hint">${p ? `Filed behind <span class="addr soft">${esc(p.address)}</span> ${esc(p.title)}` : editing ? (editing.filed ? `At <span class="addr soft">${esc(editing.address)}</span>` : 'Unfiled, on the desk') : 'A fresh card. It goes on the desk; file it when you know where it belongs.'}</div>
        <input id="ed-title" placeholder="Title" value="${esc(editing?.title || '')}" autocomplete="off">
        <textarea id="ed-body" placeholder="One idea, stated so it stands alone. Type [[ to link another card.">${esc(body)}</textarea>
        <div class="hint">⌘S / Ctrl+S saves · Esc cancels · [[ links by ID, which survives renames</div>
        <div class="actions"><button class="primary" data-act="save">Save card</button><button data-act="cancel">Cancel</button></div>
      </div>`;
  }

  // Filing: choose the card this one sits behind, or the sibling it follows.
  file({ id, drawer }) {
    const m = this.app.model, byId = this.app.byId;
    const n = byId.get(id);
    if (!n) return this.drawers();
    const head = `<div class="panel-top"><h2>${n.filed ? 'Refile' : 'File'} this card</h2>${this.close()}</div>
      <div class="filing"><div class="chip" draggable="true" id="file-chip">${esc(n.title)}</div>`;
    if (drawer == null) {
      return `${head}
        <div class="hint" style="font:12px var(--mono);color:var(--ink-2);margin-bottom:10px">Open a drawer and choose a position. Drag the card onto a slot, or click one.</div>
        ${m.boxes.map((b) => { const ds = m.drawers.filter((d) => d.box === b.id && !d.unsorted); const n = m.notes.filter((x) => x.filed && !x.parent && x.box === b.id).length; return `<div class="boxhead">${esc(b.name)}</div>` + ds.map((d) => `<div class="drawer-front" data-file-drawer="${d.index}"><span class="label">${esc(d.label)}</span><span class="count">${d.ids.length} cards</span></div>`).join('') + `<div class="rootslot" data-file-parent="" data-file-box="${b.id}" data-file-after="">${ds.length ? 'Or start' : 'Start'} a new branch in ${esc(b.name)}: ${esc(b.prefix)}${n + 1}, its own drawer.${b.id === m.boxes[0].id ? ' Most cards belong behind something.' : ' Sources and excerpts go here.'}</div>`; }).join('')}
        </div>`;
    }
    const d = m.drawers[drawer];
    const box = m.boxes.find((b) => b.id === d.box) || m.boxes[0];
    const rootsInBox = m.notes.filter((x) => x.filed && !x.parent && x.box === box.id).length;
    const rows = d.ids.filter((x) => x !== id).map((cid) => {
      const c = byId.get(cid);
      const inBranch = n.filed && this.app.isDescendant(cid, id);
      if (inBranch) return `<div class="target">${mini(c).replace('class="mini"', 'class="mini" style="opacity:.4"')}<div class="behind" title="part of the card being moved">its own branch</div></div>`;
      return `<div class="target">${mini(c)}<div class="behind" data-file-parent="${cid}" data-file-after="" title="File behind ${esc(c.address)}">behind<br>${esc(c.address)}</div></div>
        <div class="slot" data-file-parent="${c.parent || ''}" data-file-after="${cid}" data-label="after ${esc(c.address)}"></div>`;
    }).join('');
    return `${head}
      ${this.crumbs([{ text: 'Choose a drawer', act: 'filepick', arg: id }, { text: drawerName(d, m) }])}
      <div class="hint" style="font:12px var(--mono);color:var(--ink-2);margin:8px 0">Behind a card = a branch off it. After a card = next in its sequence.</div>
      ${rows || '<p class="empty">This drawer is empty. Start a branch below, or choose another drawer.</p>'}
      ${m.boxes.map((b) => { const roots = b.id === box.id ? rootsInBox : m.notes.filter((x) => x.filed && !x.parent && x.box === b.id).length; return `<div class="rootslot" data-file-parent="" data-file-box="${b.id}" data-file-after="" style="margin-top:12px">Start a new branch in ${esc(b.name)}: ${esc(b.prefix)}${roots + 1}, its own drawer</div>`; }).join('')}
      <div class="actions"><button data-act="filepick" data-arg="${id}">Choose another drawer</button></div></div>`;
  }

  desk() {
    const m = this.app.model, byId = this.app.byId;
    const cards = m.desk.map((p) => byId.get(p.id)).filter(Boolean);
    return `<div class="panel-top"><h2>On the desk</h2>${this.close()}</div>
      <div class="meta">Cards written but not yet filed. Notes created in another editor land here too. Past ${m.settings.deskCapacity} they pile up.</div>
      ${cards.map((x) => mini(x, `<button data-act="file" data-arg="${x.id}" style="margin-left:6px">file</button>`)).join('') || '<p class="empty">The desk is clear. Everything is in the box.</p>'}`;
  }

  register() {
    const m = this.app.model, byId = this.app.byId, drawerOf = this.app.drawerOf;
    const reg = m.register;
    const entry = (e) => {
      const groups = new Map();
      for (const id of e.targets) {
        const c = byId.get(id); if (!c) continue;
        const d = drawerOf.get(id);
        const key = d ? d.index : -1;
        if (!groups.has(key)) groups.set(key, { d, cards: [] });
        groups.get(key).cards.push(c);
      }
      const body = [...groups.values()].sort((a, b) => (a.d?.index ?? 99) - (b.d?.index ?? 99)).map((g) => `<div class="drawer-group"><div class="dname">${g.d && !g.d.unsorted ? `<span class="num">${drawerName(g.d, m).toLowerCase()}</span>${esc(g.d.title)}` : g.d ? 'Unsorted' : 'On the desk'}</div><div class="targets">${g.cards.map((c) => `<a data-open="${c.id}">${addrChip(c)}${esc(c.title)}<span class="rm" data-reg-rm="${esc(e.term)}" data-reg-id="${c.id}" title="Remove from term">×</span></a>`).join('')}</div></div>`).join('');
      return `<div class="ledger"><div class="term"><span>${esc(e.term)}</span><span class="x" data-reg-del="${esc(e.term)}" title="Remove term">×</span></div>${e.note ? `<div class="note">${esc(e.note)}</div>` : ''}${body || '<span class="empty">no cards yet</span>'}</div>`;
    };
    return `<div class="panel-top"><h2>The register</h2>${this.close()}</div>
      <div class="meta">Your own index: a term in your words, then the drawers and addresses where it lives. Choosing what earns an entry is part of the thinking.</div>
      <div class="addrow"><input id="reg-term" placeholder="New term…" autocomplete="off"><button data-act="register-term">Add</button></div>
      ${reg.map(entry).join('') || '<p class="empty">Empty. Open a card and press r to enter it under a term.</p>'}`;
  }

  // --- wiring ---
  onClick(e) {
    const rm = e.target.closest('[data-reg-rm]');
    if (rm) { e.stopPropagation(); this.app.registerRemove(rm.dataset.regRm, rm.dataset.regId); return; }
    const del = e.target.closest('[data-reg-del]');
    if (del) { this.app.registerRemove(del.dataset.regDel, ''); return; }
    const fd = e.target.closest('[data-file-drawer]');
    if (fd) { this.show('file', { id: this.view.arg.id, drawer: +fd.dataset.fileDrawer }); this.app.openDrawerInRoom(+fd.dataset.fileDrawer); return; }
    const fp = e.target.closest('[data-file-parent]');
    if (fp) { this.app.doFile(this.view.arg.id, fp.dataset.fileParent, fp.dataset.fileAfter, fp.dataset.fileBox || ''); return; }
    const open = e.target.closest('[data-open]');
    if (open && !e.target.closest('button')) { this.app.openCard(open.dataset.open); return; }
    const act = e.target.closest('[data-act]');
    if (!act) {
      const d = e.target.closest('[data-drawer]');
      if (d) this.app.openDrawer(+d.dataset.drawer);
      return;
    }
    const arg = act.dataset.arg;
    const a = this.app;
    ({
      close: () => a.closePanel(), back: () => this.back(), drawers: () => a.openDrawers(), drawer: () => a.openDrawer(+arg), open: () => a.openCard(arg),
      'write-behind': () => a.writeBehind(arg), desk: () => a.openDesk(),
      pin: () => a.pin(arg), edit: () => a.edit(arg), file: () => a.fileCard(arg), filepick: () => a.fileCard(arg, true), trash: () => a.trash(arg),
      save: () => this.save(), cancel: () => a.cancelEdit(),
      board: () => a.openBoard(arg), register: () => a.openRegister(),
      'register-add': () => a.registerAdd(arg), 'register-term': () => this.addTerm(),
    }[act.dataset.act] || (() => {}))();
  }

  wireFiling() {
    const chip = this.el.querySelector('#file-chip');
    if (!chip) return;
    chip.addEventListener('dragstart', (e) => { e.dataTransfer.setData('text/plain', 'card'); e.dataTransfer.effectAllowed = 'move'; });
    for (const t of this.el.querySelectorAll('[data-file-parent]')) {
      t.addEventListener('dragover', (e) => { e.preventDefault(); t.classList.add('over'); });
      t.addEventListener('dragleave', () => t.classList.remove('over'));
      t.addEventListener('drop', (e) => { e.preventDefault(); t.classList.remove('over'); this.app.doFile(this.view.arg.id, t.dataset.fileParent, t.dataset.fileAfter, t.dataset.fileBox || ''); });
    }
  }

  wireRegister() {
    const input = this.el.querySelector('#reg-term');
    input?.addEventListener('keydown', (e) => { if (e.key === 'Enter') this.addTerm(); });
  }
  addTerm() {
    const input = this.el.querySelector('#reg-term');
    if (input?.value.trim()) this.app.registerAddTerm(input.value.trim());
  }

  wireEditor({ id, parent }) {
    const title = this.el.querySelector('#ed-title'), body = this.el.querySelector('#ed-body');
    (id ? body : title).focus();
    const keys = (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') { e.preventDefault(); this.save(); }
      if (e.key === 'Escape' && !this.ac) { e.preventDefault(); this.app.cancelEdit(); }
    };
    title.addEventListener('keydown', keys);
    body.addEventListener('keydown', (e) => { if (this.acKey(e)) return; keys(e); });
    body.addEventListener('input', () => this.autocomplete(body));
    body.addEventListener('blur', () => setTimeout(() => this.closeAc(), 150));
    this.editing = { id, parent, seen: id && this.app.detail?.id === id ? this.app.detail.modTime : undefined };
  }

  dirty() {
    const title = this.el.querySelector('#ed-title'), body = this.el.querySelector('#ed-body');
    return !!(title && (title.value.trim() || body.value.trim()));
  }

  save() {
    const title = this.el.querySelector('#ed-title')?.value ?? '', body = this.el.querySelector('#ed-body')?.value ?? '';
    this.app.saveCard({ ...this.editing, title, body });
  }

  // Autocomplete for [[ links: inserts the ID form with the title as alias.
  autocomplete(ta) {
    const upto = ta.value.slice(0, ta.selectionStart);
    const m = upto.match(/\[\[([^\]|\n]*)$/);
    if (!m) { this.closeAc(); return; }
    const q = m[1].toLowerCase();
    const hits = this.app.model.notes.filter((n) => !n.guest && (n.title.toLowerCase().includes(q) || n.address.toLowerCase() === q)).slice(0, 8);
    if (!hits.length) { this.closeAc(); return; }
    if (!this.ac) { this.ac = document.createElement('div'); this.ac.className = 'ac'; ta.parentElement.appendChild(this.ac); this.acSel = 0; }
    this.acHits = hits; this.acStart = ta.selectionStart - m[1].length;
    this.acSel = Math.min(this.acSel, hits.length - 1);
    this.ac.innerHTML = hits.map((n, i) => `<div class="${i === this.acSel ? 'sel' : ''}" data-i="${i}">${addrChip(n)}${esc(n.title)}</div>`).join('');
    this.ac.style.top = (ta.offsetTop + 40) + 'px'; this.ac.style.left = ta.offsetLeft + 'px';
    this.ac.onmousedown = (e) => { const d = e.target.closest('[data-i]'); if (d) { e.preventDefault(); this.pickAc(ta, +d.dataset.i); } };
  }
  acKey(e) {
    if (!this.ac) return false;
    const ta = e.target;
    if (e.key === 'ArrowDown') { this.acSel = (this.acSel + 1) % this.acHits.length; this.autocomplete(ta); e.preventDefault(); return true; }
    if (e.key === 'ArrowUp') { this.acSel = (this.acSel - 1 + this.acHits.length) % this.acHits.length; this.autocomplete(ta); e.preventDefault(); return true; }
    if (e.key === 'Enter' || e.key === 'Tab') { this.pickAc(ta, this.acSel); e.preventDefault(); return true; }
    if (e.key === 'Escape') { this.closeAc(); e.preventDefault(); return true; }
    return false;
  }
  pickAc(ta, i) {
    const n = this.acHits[i]; if (!n) return;
    const before = ta.value.slice(0, this.acStart), after = ta.value.slice(ta.selectionStart);
    const closing = after.startsWith(']]') ? after.slice(2) : after;
    const link = `${n.id}|${n.title}]]`;
    ta.value = before + link + closing;
    const pos = before.length + link.length;
    ta.setSelectionRange(pos, pos);
    this.closeAc(); ta.focus();
  }
  closeAc() { if (this.ac) { this.ac.remove(); this.ac = null; } }
}
