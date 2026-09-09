// The room: a small pixel scene, scaled by whole numbers, with hit regions
// that map clicks to objects. It renders whatever model it is given and
// reports intent upward; it never talks to the server itself.
import {
  P, SCENE, L, BOX_DRAWERS, boxPages, outline, daylight, makeVignette,
  drawRoomBase, drawWallShelf, drawCorkboard, drawDesk, drawInbox, drawLedger, drawLamp,
  drawSlipbox, makeGlow, drawFlyingCard,
} from './art.js';

export const BOARD = { w: 1200, h: 700 }; // logical corkboard space used by the full-screen board

const ease = (t) => 1 - Math.pow(1 - t, 3);

export class Room {
  constructor(canvas, handlers) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
    this.h = handlers;
    this.scene = document.createElement('canvas');
    this.scene.width = SCENE.w; this.scene.height = SCENE.h;
    this.sctx = this.scene.getContext('2d');
    this.glow = makeGlow(96);
    this.vignette = makeVignette(SCENE.w, SCENE.h);
    this.scale = 2;
    this.camX = 40; this.targetCam = 40;
    this.lampOn = true;
    this.drawerCount = 1; this.pageMain = 0; this.pageLit = 0;
    this.inboxCount = 0; this.inboxOver = false; this.hasRedis = false; this.pins = [];
    this.hover = null; this.pressed = null;
    this.openDrawer = -1; this.openAmt = 0; this.openTarget = 0;
    this.highlight = null;
    this.flights = [];
    this.hourOverride = null;
    this.bind();
    this.viewW = SCENE.view;
    this.resize();
    new ResizeObserver(() => this.resize()).observe(canvas.parentElement);
    requestAnimationFrame((t) => this.loop(t));
  }

  resize() {
    const stage = this.canvas.parentElement;
    const availW = stage.clientWidth, availH = stage.clientHeight;
    // Scale from the height, so opening the panel narrows the view instead
    // of shrinking the whole room; only drop a step when the strip left
    // would be too thin to use.
    this.scale = Math.max(1, Math.floor(availH / SCENE.h));
    while (this.scale > 1 && availW / this.scale < SCENE.minView) this.scale--;
    this.viewW = Math.max(SCENE.minView, Math.min(SCENE.w, Math.floor(availW / this.scale)));
    this.camX = Math.min(this.camX, SCENE.w - this.viewW);
    this.targetCam = Math.min(this.targetCam, SCENE.w - this.viewW);
    if (this.panKind) this.panToObject(this.panKind); // the view changed shape: aim at the same object
    this.canvas.width = this.viewW * this.scale;
    this.canvas.height = SCENE.h * this.scale;
    this.canvas.style.width = this.canvas.width + 'px';
    this.canvas.style.height = this.canvas.height + 'px';
    this.ctx.imageSmoothingEnabled = false;
  }

  setModel(m, activeBoardId) {
    this.model = m;
    this.drawers = m.drawers;
    const mainBox = m.boxes?.[0]?.id || 'main';
    this.mainDrawers = m.drawers.filter((d) => !d.unsorted && d.box === mainBox);
    this.litDrawers = m.drawers.filter((d) => !d.unsorted && d.box !== mainBox);
    this.drawerCount = this.mainDrawers.length;
    this.pageMain = Math.min(this.pageMain, boxPages(this.mainDrawers.length) - 1);
    this.pageLit = Math.min(this.pageLit, boxPages(this.litDrawers.length) - 1);
    // The desk is the inbox: every unfiled card lies in the tray as one
    // stack. Arranging them happens in the desk view, not here.
    this.inboxCount = m.desk.length;
    this.inboxOver = m.desk.length > (m.settings.deskCapacity || 12);
    this.hasRedis = !!m.rediscover && m.prefs?.rediscoverAway !== m.rediscover;
    this.registerCount = (m.register || []).length;
    const board = m.boards.find((b) => b.id === activeBoardId) || m.boards[0];
    this.pins = board ? board.cards.map((c) => ({ x: c.x / BOARD.w * 400, y: c.y / BOARD.h * 300 })) : [];
    if (typeof m.prefs?.lamp === 'boolean') this.lampOn = m.prefs.lamp;
    if (typeof m.prefs?.camera === 'number') { this.camX = this.targetCam = m.prefs.camera; }
  }

  hour() {
    if (this.hourOverride != null) return this.hourOverride;
    const d = new Date();
    return d.getHours() + d.getMinutes() / 60;
  }

  // Hit regions, top-most first.
  objects() {
    const list = [];
    for (const dr of this.drawerRects || []) {
      const d = this.drawers?.[dr.index];
      if (d) list.push({ kind: 'drawer', index: dr.index, label: `Drawer ${d.number} · ${d.ids.length} card${d.ids.length === 1 ? '' : 's'}`, rect: dr });
    }
    list.push({ kind: 'pad', label: 'Blank cards: write a new card', rect: L.pad });
    for (const lr of this.litRects || []) {
      const d = this.drawers?.[lr.index];
      if (d) list.push({ kind: 'drawer', index: lr.index, label: `Literature drawer ${d.number} · ${d.ids.length} card${d.ids.length === 1 ? '' : 's'}`, rect: lr });
    }
    const litN = this.litDrawers?.length || 0, litPages = boxPages(litN);
    list.push({ kind: 'litbox', label: litN ? `Literature box: ${litN} drawer${litN === 1 ? '' : 's'}${litPages > 1 ? ` · box ${this.pageLit + 1} of ${litPages}, click for the next` : ''}` : 'Literature box: empty. File a source card here to start it.', rect: L.boxLit });
    list.push({ kind: 'ledger', label: `Register: ${this.registerCount} term${this.registerCount === 1 ? '' : 's'}`, rect: { x: L.ledger.x - 1, y: L.ledger.y - 2, w: L.ledger.w + 2, h: L.ledger.h + 3 } });
    list.push({ kind: 'contents', label: 'Contents: which number is which drawer', rect: { x: L.contents.x - 2, y: L.contents.y - 2, w: L.contents.w + 4, h: L.contents.h + 4 } });
    list.push({ kind: 'lamp', label: this.lampOn ? 'Lamp: turn off' : 'Lamp: turn on', rect: L.lamp });
    const total = this.model ? this.model.notes.filter((n) => n.filed).length : 0;
    const pages = boxPages(this.drawerCount);
    list.push({ kind: 'slipbox', label: `Slip box: ${this.drawerCount} drawer${this.drawerCount === 1 ? '' : 's'}, ${total} cards filed${pages > 1 ? ` · box ${this.pageMain + 1} of ${pages}, click for the next` : ''}`, rect: L.boxMain });
    const boards = this.model ? this.model.boards.length : 0;
    list.push({ kind: 'cork', label: `Corkboard: ${boards} board${boards === 1 ? '' : 's'}`, rect: { x: L.cork.x - 4, y: L.cork.y - 4, w: L.cork.w + 8, h: L.cork.h + 8 } });
    const waiting = this.inboxCount ? `${this.inboxCount} card${this.inboxCount === 1 ? '' : 's'} waiting to be filed${this.hasRedis ? ', and one slipped out of the box today' : ''}` : 'clear';
    list.push({ kind: 'inbox', label: `Inbox: ${waiting}`, rect: { x: L.inbox.x - 1, y: L.inbox.y - 4, w: L.inbox.w + 2, h: L.inbox.h + 5 } });
    list.push({ kind: 'desk', label: `Desk: ${waiting}`, rect: L.desk });
    return list;
  }

  hit(sx, sy) {
    for (const o of this.objects()) {
      const r = o.rect;
      if (sx >= r.x && sx < r.x + r.w && sy >= r.y && sy < r.y + r.h) return o;
    }
    return null;
  }

  scenePoint(e) {
    const rect = this.canvas.getBoundingClientRect();
    return { x: (e.clientX - rect.left) / this.scale + this.camX, y: (e.clientY - rect.top) / this.scale };
  }

  bind() {
    const cv = this.canvas;
    cv.addEventListener('pointerdown', (e) => {
      const p = this.scenePoint(e);
      const o = this.hit(p.x, p.y);
      this.pressed = { x: e.clientX, y: e.clientY, obj: o, cam: this.camX, moved: false };
      cv.setPointerCapture(e.pointerId);
    });
    cv.addEventListener('pointermove', (e) => {
      const p = this.scenePoint(e);
      if (this.pressed) {
        const dx = e.clientX - this.pressed.x, dy = e.clientY - this.pressed.y;
        if (Math.abs(dx) + Math.abs(dy) > 3) this.pressed.moved = true;
        if (this.pressed.moved) {
          this.panKind = null;
          this.camX = this.targetCam = Math.max(0, Math.min(SCENE.w - this.viewW, this.pressed.cam - dx / this.scale));
          cv.className = 'grabbing';
          return;
        }
      }
      const o = this.hit(p.x, p.y);
      if (o !== this.hover && (o?.kind !== this.hover?.kind || o?.id !== this.hover?.id || o?.index !== this.hover?.index)) {
        this.hover = o;
        this.h.onHover?.(o);
      }
      cv.className = o ? 'pointer' : '';
    });
    const up = (e) => {
      if (!this.pressed) return;
      const pressed = this.pressed; this.pressed = null;
      if (pressed.moved) { this.h.onCamera?.(this.camX); cv.className = ''; return; }
      if (pressed.obj) this.h.onClick?.(pressed.obj);
    };
    cv.addEventListener('pointerup', up);
    cv.addEventListener('pointercancel', up);
    cv.addEventListener('pointerleave', () => { if (!this.pressed) { this.hover = null; this.h.onHover?.(null); cv.className = ''; } });
  }

  panTo(x) { this.panKind = null; this.targetCam = Math.max(0, Math.min(SCENE.w - this.viewW, x)); }
  panToObject(kind) {
    this.panKind = kind;
    const r = kind === 'cork' ? L.cork : kind === 'slipbox' ? L.boxMain : L.desk;
    this.targetCam = Math.max(0, Math.min(SCENE.w - this.viewW, r.x + r.w / 2 - this.viewW / 2 + (kind === 'slipbox' ? 40 : kind === 'cork' ? -30 : 0)));
  }

  // Which box and page a drawer index lives in, so the room can show it.
  locate(i) {
    let j = (this.mainDrawers || []).findIndex((d) => d.index === i);
    if (j >= 0) return { box: 'main', page: (j / BOX_DRAWERS) | 0, slot: j % BOX_DRAWERS };
    j = (this.litDrawers || []).findIndex((d) => d.index === i);
    if (j >= 0) return { box: 'lit', page: (j / BOX_DRAWERS) | 0, slot: j % BOX_DRAWERS };
    return null;
  }
  openDrawerAnim(i) {
    const at = this.locate(i);
    if (at) { if (at.box === 'main') this.pageMain = at.page; else this.pageLit = at.page; }
    this.openDrawer = i; this.openTarget = 1;
  }
  // Cycle to the next box of a stack; returns { page, pages } after the turn.
  nextPage(box) {
    const n = box === 'main' ? (this.mainDrawers || []).length : (this.litDrawers || []).length;
    const pages = boxPages(n);
    if (box === 'main') this.pageMain = (this.pageMain + 1) % pages; else this.pageLit = (this.pageLit + 1) % pages;
    return { page: box === 'main' ? this.pageMain : this.pageLit, pages };
  }
  closeDrawer() { this.openTarget = 0; }

  // A card flies from a scene rect to a screen-space point (e.g. the panel).
  fly(from, toScene) {
    this.flights.push({ x0: from.x, y0: from.y, x1: toScene.x, y1: toScene.y, start: performance.now(), dur: 320 });
  }
  flyFromSlipbox() {
    const r = L.boxMain;
    this.fly({ x: r.x + r.w / 2, y: r.y + r.h / 2 }, { x: this.targetCam + this.viewW - 20, y: 60 });
  }

  loop(t) {
    // camera and drawer tweens
    this.camX += (this.targetCam - this.camX) * 0.15;
    if (Math.abs(this.targetCam - this.camX) < 0.2) this.camX = this.targetCam;
    this.openAmt += (this.openTarget - this.openAmt) * 0.22;
    if (Math.abs(this.openTarget - this.openAmt) < 0.02) { this.openAmt = this.openTarget; if (this.openAmt === 0) this.openDrawer = -1; }
    this.render(t);
    requestAnimationFrame((tt) => this.loop(tt));
  }

  render(t) {
    const s = this.sctx;
    const day = daylight(this.hour());
    drawRoomBase(s);
    drawWallShelf(s, this.hover?.kind === 'contents');
    drawCorkboard(s, this.pins, this.highlight === 'cork');
    drawDesk(s);
    const at = this.openDrawer >= 0 ? this.locate(this.openDrawer) : null;
    const slotIn = (box, page) => at && at.box === box && at.page === page ? at.slot : -1;
    this.drawerRects = drawSlipbox(s, L.boxMain, this.mainDrawers || [], this.pageMain, slotIn('main', this.pageMain), this.openAmt, this.highlight === 'slipbox', false);
    this.litRects = drawSlipbox(s, L.boxLit, this.litDrawers || [], this.pageLit, slotIn('lit', this.pageLit), this.openAmt, this.hover?.kind === 'litbox', true);
    if (this.hover?.kind === 'drawer') { const dr = [...this.drawerRects, ...this.litRects].find((r) => r.index === this.hover.index); if (dr) outline(s, dr.x - 1, dr.y - 1, dr.w + 2, dr.h + 2, P.yellow); }
    drawLamp(s, this.lampOn);
    drawInbox(s, this.inboxCount, this.inboxOver, this.hasRedis, this.hover?.kind === 'inbox');
    drawLedger(s, this.hover?.kind === 'ledger');
    if (this.hover?.kind === 'pad') outline(s, L.pad.x - 2, L.pad.y - 2, L.pad.w + 4, L.pad.h + 4, P.yellow);
    if (this.hover?.kind === 'lamp') outline(s, L.lamp.x, L.lamp.y - 2, L.lamp.w, L.lamp.h + 2, P.yellow);
    if (this.hover?.kind === 'slipbox' && this.highlight !== 'slipbox') { const r = L.boxMain; outline(s, r.x - 2, r.y - 2, r.w + 4, r.h + 4, P.cream); }
    if (this.hover?.kind === 'desk') outline(s, L.desk.x - 1, L.desk.y - 1, L.desk.w + 2, L.desk.h + 2, P.cream);
    if (this.hover?.kind === 'cork' && this.highlight !== 'cork') outline(s, L.cork.x - 5, L.cork.y - 5, L.cork.w + 10, L.cork.h + 10, P.cream);

    // a library is dim even by day; night deepens it a little, and the lamp
    // pushes it back. Capped so the room stays readable at midnight.
    const dark = 0.2 + (1 - day) * 0.18;
    s.fillStyle = `rgba(24, 20, 50, ${dark.toFixed(2)})`; s.fillRect(0, 0, SCENE.w, SCENE.h);
    s.drawImage(this.vignette, 0, 0);
    s.globalCompositeOperation = 'lighter';
    if (this.lampOn) { s.globalAlpha = 0.45 + (1 - day) * 0.35; s.drawImage(this.glow, L.lampGlow.x - 96, L.lampGlow.y - 96); }
    s.globalAlpha = 1;
    s.globalCompositeOperation = 'source-over';
    // flights
    const now = performance.now();
    this.flights = this.flights.filter((f) => now - f.start < f.dur);
    for (const f of this.flights) {
      const k = ease((now - f.start) / f.dur);
      drawFlyingCard(s, Math.round(f.x0 + (f.x1 - f.x0) * k), Math.round(f.y0 + (f.y1 - f.y0) * k - Math.sin(k * Math.PI) * 30));
    }
    // blit the viewport
    const c = this.ctx;
    c.imageSmoothingEnabled = false;
    c.drawImage(this.scene, Math.round(this.camX), 0, this.viewW, SCENE.h, 0, 0, this.canvas.width, this.canvas.height);
  }
}
