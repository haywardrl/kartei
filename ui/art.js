// Procedural pixel art for the room. Everything here draws into a small scene
// canvas that is scaled up by whole numbers, so all coordinates are integers.
// Each function is one "layer" that a hand-drawn PNG could replace later.

// Endesga 32 palette, locked. Working inside a fixed palette is what makes
// programmer art read as intentional.
export const P = {
  rust: '#be4a2f', orange: '#d77643', cream: '#ead4aa', tan: '#e4a672', brown: '#b86f50',
  umber: '#733e39', darkumber: '#3e2731', crimson: '#a22633', red: '#e43b44', amber: '#f77622',
  gold: '#feae34', yellow: '#fee761', lime: '#63c74d', green: '#3e8948', forest: '#265c42',
  pine: '#193c3e', navy: '#124e89', sky: '#0099db', cyan: '#2ce8f5', white: '#ffffff',
  fog: '#c0cbdc', steel: '#8b9bb4', slate: '#5a6988', dusk: '#3a4466', night: '#262b44',
  ink: '#181425', pink: '#ff0044', plum: '#68386c', mauve: '#b55088', rose: '#f6757a',
  sand: '#e8b796', clay: '#c28569',
};

export const SCENE = { w: 560, h: 270, view: 480, minView: 220 };

// Layout of every object in scene coordinates. The room can be redrawn but
// these rectangles are also the hit regions, so keep them in one place.
export const L = {
  wallBottom: 188,
  cork:    { x: 24,  y: 36,  w: 160, h: 104 },
  picture: { x: 250, y: 38,  w: 48,  h: 36 },   // a painting above the desk
  wallShelf: { x: 212, y: 100, w: 96, h: 5 },   // a plank on the wall, with brackets
  plant:   { x: 218, y: 70,  w: 24,  h: 30 },   // on the wall shelf
  contents: { x: 270, y: 82, w: 22,  h: 18 },   // the contents ledger, standing on the shelf
  desk:    { x: 200, y: 140, w: 160, h: 26 },
  deskFront: { x: 200, y: 166, w: 160, h: 16 },
  inbox:   { x: 214, y: 144, w: 44,  h: 16 },   // the tray: every unfiled card, as a stack
  pad:     { x: 268, y: 146, w: 20,  h: 12 },   // a stack of blank cards
  lamp:    { x: 318, y: 96,  w: 34,  h: 44 },
  lampGlow: { x: 335, y: 112 },
  ledger:  { x: 294, y: 148, w: 18,  h: 12 },   // the register: your own index of terms
  boxMain: { x: 380, y: 76,  w: 80,  h: 120 },  // the slip box for notes, on the floor against the wall
  boxLit:  { x: 470, y: 76,  w: 80,  h: 120 },  // the slip box for literature, green
};

// Desk-card positions are stored in this logical space (the old room desk,
// 98x22 minus a 14x10 card). The desk view maps it to its big desk, so
// saved layouts survive the room desk changing shape.
export const DESK_SPACE = { w: 84, h: 12 };

// --- primitives ---
export function px(ctx, x, y, w, h, c) { ctx.fillStyle = c; ctx.fillRect(x | 0, y | 0, w | 0, h | 0); }
export function dither(ctx, x, y, w, h, a, b) {
  px(ctx, x, y, w, h, a);
  ctx.fillStyle = b;
  for (let j = 0; j < h; j++) for (let i = (j & 1); i < w; i += 2) ctx.fillRect(x + i, y + j, 1, 1);
}
export function outline(ctx, x, y, w, h, c) {
  px(ctx, x, y, w, 1, c); px(ctx, x, y + h - 1, w, 1, c); px(ctx, x, y, 1, h, c); px(ctx, x + w - 1, y, 1, h, c);
}
function hline(ctx, x, y, w, c) { px(ctx, x, y, w, 1, c); }

// --- time of day ---
export function daylight(hour) {
  // 0 = full dark, 1 = full day
  if (hour >= 8 && hour <= 17) return 1;
  if (hour >= 21 || hour < 5) return 0;
  if (hour < 8) return (hour - 5) / 3;
  return 1 - (hour - 17) / 4;
}

// --- layers ---
export function drawRoomBase(ctx) {
  // dark wood panelling, the library's wall
  px(ctx, 0, 0, SCENE.w, L.wallBottom, '#3a2a30');
  for (let x = 0; x < SCENE.w; x += 16) { px(ctx, x, 0, 1, L.wallBottom, '#2e2126'); px(ctx, x + 8, 0, 1, L.wallBottom, '#41303a'); }
  hline(ctx, 0, 22, SCENE.w, P.umber); hline(ctx, 0, 23, SCENE.w, P.darkumber); // picture rail
  hline(ctx, 0, L.wallBottom - 6, SCENE.w, P.umber);
  px(ctx, 0, L.wallBottom - 5, SCENE.w, 5, P.darkumber);
  // floor planks, dark and worn
  for (let y = L.wallBottom; y < SCENE.h; y += 8) {
    px(ctx, 0, y, SCENE.w, 8, (y / 8) % 2 ? '#4a3024' : '#503527');
    hline(ctx, 0, y, SCENE.w, '#33201a');
    const off = ((y / 8) | 0) % 3 * 40;
    for (let x = off; x < SCENE.w; x += 120) px(ctx, x, y + 1, 1, 7, '#33201a');
  }
  drawPicture(ctx);
}

// A painting above the desk: hills under a pale sky, in a gilt frame.
function drawPicture(ctx) {
  const p = L.picture;
  px(ctx, p.x + 2, p.y + 2, p.w, p.h, '#2e2126');
  px(ctx, p.x - 3, p.y - 3, p.w + 6, p.h + 6, P.gold);
  outline(ctx, p.x - 3, p.y - 3, p.w + 6, p.h + 6, P.amber);
  px(ctx, p.x - 1, p.y - 1, p.w + 2, p.h + 2, P.umber);
  px(ctx, p.x, p.y, p.w, p.h, P.sand);
  dither(ctx, p.x, p.y, p.w, 10, P.sand, P.cream);
  px(ctx, p.x + 30, p.y + 5, 6, 6, P.gold); px(ctx, p.x + 31, p.y + 4, 4, 8, P.gold);
  px(ctx, p.x, p.y + 16, p.w, p.h - 16, P.forest);
  px(ctx, p.x, p.y + 14, 20, 4, P.forest); px(ctx, p.x + 14, p.y + 12, 18, 6, P.green); px(ctx, p.x + 30, p.y + 15, 18, 3, P.forest);
  dither(ctx, p.x, p.y + 22, p.w, p.h - 22, P.forest, P.pine);
  px(ctx, p.x + 20, p.y + 24, 3, 8, P.umber); px(ctx, p.x + 17, p.y + 18, 9, 8, P.pine);
}

// The wall shelf above the desk, with the plant and the contents ledger on it.
export function drawWallShelf(ctx, hoverContents) {
  const w = L.wallShelf;
  px(ctx, w.x, w.y + w.h, w.w, 3, 'rgba(0,0,0,.35)');
  px(ctx, w.x, w.y, w.w, w.h, '#8f5637'); hline(ctx, w.x, w.y, w.w, '#b8744a'); hline(ctx, w.x, w.y + w.h - 1, w.w, '#4a2a1c');
  for (const bx of [w.x + 8, w.x + w.w - 12]) { px(ctx, bx, w.y + w.h, 4, 8, '#5c3223'); px(ctx, bx, w.y + w.h, 1, 8, '#7a4630'); }
  drawPlant(ctx);
  const c = L.contents;
  px(ctx, c.x + 1, c.y + 1, c.w, c.h, '#2e2126');
  px(ctx, c.x, c.y, c.w, c.h, P.crimson);
  px(ctx, c.x, c.y, 3, c.h, '#7a1f2b');
  px(ctx, c.x + 6, c.y + 4, c.w - 9, 6, P.cream);
  hline(ctx, c.x + 8, c.y + 6, 6, P.slate); hline(ctx, c.x + 8, c.y + 8, 4, P.slate);
  px(ctx, c.x + c.w - 2, c.y + 1, 1, c.h - 2, P.cream);
  if (hoverContents) outline(ctx, c.x - 2, c.y - 2, c.w + 4, c.h + 4, P.yellow);
}

function drawPlant(ctx) {
  const p = L.plant;
  px(ctx, p.x + 6, p.y + 20, 12, 10, P.orange); px(ctx, p.x + 4, p.y + 18, 16, 3, P.rust);
  px(ctx, p.x + 7, p.y + 21, 2, 8, P.tan);
  const leaf = [[8, 8, 6, 12], [2, 6, 7, 8], [14, 4, 7, 10], [6, 0, 5, 10], [0, 12, 6, 6], [18, 12, 6, 6]];
  for (const [x, y, w, h] of leaf) { px(ctx, p.x + x, p.y + y, w, h, P.forest); px(ctx, p.x + x + 1, p.y + y + 1, w - 2, 2, P.green); }
}

export function drawCorkboard(ctx, pins, highlight) {
  const c = L.cork;
  px(ctx, c.x - 4, c.y - 4, c.w + 8, c.h + 8, P.umber);
  px(ctx, c.x - 3, c.y - 3, c.w + 6, c.h + 6, P.brown);
  dither(ctx, c.x, c.y, c.w, c.h, P.clay, P.brown);
  // a few darker cork flecks
  for (let i = 0; i < 40; i++) { const fx = (i * 37) % c.w, fy = (i * 53) % c.h; px(ctx, c.x + fx, c.y + fy, 1, 1, P.umber); }
  // pinned cards, mapped from the 400x300 board space
  for (const p of pins) {
    const x = c.x + 4 + Math.round(p.x / 400 * (c.w - 24)), y = c.y + 4 + Math.round(p.y / 300 * (c.h - 18));
    px(ctx, x + 1, y + 1, 16, 12, P.slate);
    px(ctx, x, y, 16, 12, P.white);
    hline(ctx, x + 2, y + 3, 10, P.rose); hline(ctx, x + 2, y + 6, 12, P.fog); hline(ctx, x + 2, y + 8, 8, P.fog);
    px(ctx, x + 7, y - 1, 2, 2, P.red);
  }
  if (highlight) outline(ctx, c.x - 5, c.y - 5, c.w + 10, c.h + 10, P.yellow);
}

// Wood grain: short darker and lighter strokes in a fixed pattern, so it
// reads as timber rather than a flat rectangle and never flickers.
function grain(ctx, x, y, w, h, dark, light, seed = 0) {
  let i = seed;
  for (let yy = y + 1; yy < y + h - 1; yy += 2) {
    let xx = x + (i * 7) % 11;
    while (xx < x + w - 3) {
      const len = 4 + (i * 13) % 14;
      px(ctx, xx, yy, Math.min(len, x + w - 1 - xx), 1, (i % 3 === 0) ? light : dark);
      xx += len + 6 + (i * 5) % 9;
      i += 1;
    }
    i += 3;
  }
}

export function drawDesk(ctx) {
  const d = L.desk, f = L.deskFront;
  px(ctx, d.x + 4, d.y - 4, d.w, 4, '#2e2126'); // shadow on the wall
  // legs
  px(ctx, d.x + 6, f.y + f.h, 8, 32, '#5a3324'); px(ctx, d.x + d.w - 14, f.y + f.h, 8, 32, '#5a3324');
  px(ctx, d.x + 6, f.y + f.h, 2, 32, '#3e2119'); px(ctx, d.x + d.w - 14, f.y + f.h, 2, 32, '#3e2119');
  px(ctx, d.x + 8, f.y + f.h, 1, 32, '#7a4a30'); px(ctx, d.x + d.w - 12, f.y + f.h, 1, 32, '#7a4a30');
  // top: warm timber with grain, a lit front edge and a darker back
  px(ctx, d.x, d.y, d.w, d.h, '#8f5637');
  grain(ctx, d.x, d.y, d.w, d.h, '#7c4830', '#a6673f', 3);
  hline(ctx, d.x, d.y, d.w, '#b8744a');
  hline(ctx, d.x, d.y + 1, d.w, '#9d5f3c');
  px(ctx, d.x, d.y + d.h - 2, d.w, 2, '#6e3f2a');
  // front apron with a drawer and a brass pull
  px(ctx, f.x, f.y, f.w, f.h, '#6a3c29');
  grain(ctx, f.x, f.y, f.w, f.h, '#5c3223', '#7a4630', 11);
  hline(ctx, f.x, f.y, f.w, '#4a2a1c');
  px(ctx, f.x + 55, f.y + 3, 50, 10, '#7a4630'); outline(ctx, f.x + 55, f.y + 3, 50, 10, '#3e2119');
  hline(ctx, f.x + 56, f.y + 4, 48, '#8f5637');
  px(ctx, f.x + 77, f.y + 7, 6, 2, P.gold); px(ctx, f.x + 77, f.y + 7, 6, 1, P.yellow);
  // a stack of blank cards
  const p = L.pad;
  px(ctx, p.x + 1, p.y + 2, p.w, p.h - 1, P.slate);
  for (let i = p.h - 6; i >= 0; i--) { px(ctx, p.x, p.y + i, p.w, p.h - 5, P.fog); px(ctx, p.x, p.y + i, p.w, 1, P.white); }
  px(ctx, p.x, p.y, p.w, p.h - 5, P.white); hline(ctx, p.x + 2, p.y + 3, p.w - 4, P.rose);
}

// The register: a small green ledger on the desk, your own index of terms.
export function drawLedger(ctx, hover) {
  const r = L.ledger;
  px(ctx, r.x + 1, r.y + 1, r.w, r.h, '#4f3f47');
  px(ctx, r.x, r.y, r.w, r.h, P.forest);
  px(ctx, r.x + 2, r.y, 2, r.h, P.pine);
  px(ctx, r.x + 6, r.y + 3, r.w - 9, 4, P.cream);
  hline(ctx, r.x + 7, r.y + 4, 6, P.slate);
  px(ctx, r.x + r.w - 2, r.y + 1, 1, r.h - 2, P.cream);
  if (hover) outline(ctx, r.x - 2, r.y - 2, r.w + 4, r.h + 4, P.yellow);
}

// The inbox: a tray on the desk holding every unfiled card as one stack.
// A red tab means a card slipped out of the box today; an orange pip means
// the stack is past the desk cap. Click it for the desk in detail.
export function drawInbox(ctx, count, over, hasRedis, hover) {
  const r = L.inbox;
  px(ctx, r.x + 1, r.y + 2, r.w, r.h, '#4f3f47');
  px(ctx, r.x, r.y, r.w, r.h, P.umber);
  px(ctx, r.x + 2, r.y + 2, r.w - 4, r.h - 3, P.darkumber);
  hline(ctx, r.x, r.y, r.w, P.brown);
  const n = Math.min(count, 8);
  for (let i = 0; i < n; i++) {
    const y = r.y + r.h - 4 - i;
    px(ctx, r.x + 4, y, r.w - 8, 1, i === n - 1 ? P.cream : (i & 1) ? P.tan : P.cream);
  }
  if (n > 0) {
    const top = r.y + r.h - 3 - n;
    px(ctx, r.x + 4, top, r.w - 8, 2, P.cream);
    hline(ctx, r.x + 7, top + 1, 10, P.clay);
    if (hasRedis) px(ctx, r.x + r.w - 10, top - 3, 3, 4, P.red);
  }
  if (over) { px(ctx, r.x + r.w - 5, r.y - 2, 2, 2, P.orange); }
  if (hover) outline(ctx, r.x - 2, r.y - 2, r.w + 4, r.h + 4, P.yellow);
}

export function drawLamp(ctx, on) {
  const l = L.lamp;
  // base and stem
  px(ctx, l.x + 6, l.y + 38, 22, 4, P.dusk); px(ctx, l.x + 8, l.y + 36, 18, 2, P.slate);
  px(ctx, l.x + 16, l.y + 12, 3, 26, P.dusk);
  // shade (trapezoid)
  for (let i = 0; i < 14; i++) px(ctx, l.x + 10 - (i >> 1), l.y + i, 14 + i, 1, on ? P.gold : P.umber);
  px(ctx, l.x + 4, l.y + 13, 26, 2, on ? P.amber : P.darkumber);
  if (on) { px(ctx, l.x + 12, l.y + 15, 10, 2, P.yellow); }
}

// A 3x5 pixel digit font for drawer numbers.
const DIGITS = {
  '0': ['111', '101', '101', '101', '111'], '1': ['010', '110', '010', '010', '111'],
  '2': ['111', '001', '111', '100', '111'], '3': ['111', '001', '111', '001', '111'],
  '4': ['101', '101', '111', '001', '001'], '5': ['111', '100', '111', '001', '111'],
  '6': ['111', '100', '111', '101', '111'], '7': ['111', '001', '010', '010', '010'],
  '8': ['111', '101', '111', '101', '111'], '9': ['111', '101', '111', '001', '111'],
};
export function drawDigits(ctx, x, y, text, color) {
  let cx = x;
  for (const ch of String(text)) {
    const g = DIGITS[ch]; if (!g) continue;
    for (let row = 0; row < 5; row++) for (let col = 0; col < 3; col++) if (g[row][col] === '1') px(ctx, cx + col, y + row, 1, 1, color);
    cx += 4;
  }
  return cx - x - 1;
}
export const digitsWidth = (text) => String(text).length * 4 - 1;

// A slip box: a fixed card-index box with eight drawer fronts, two across and
// four down. Past eight drawers the archive is more boxes: page k shows the
// next eight, and pips on the lid say which box you are looking at.
// Returns the drawer rects so each front is clickable.
export const BOX_DRAWERS = 8;
export const boxPages = (n) => Math.max(1, Math.ceil(n / BOX_DRAWERS));
export function drawSlipbox(ctx, r, drawers, page, openSlot, openAmt, highlight, green) {
  const pages = boxPages(drawers.length);
  const body = green ? P.forest : '#6a3c29', dark = green ? P.pine : '#5c3223', light = green ? P.green : '#7a4630', lid = green ? '#3e8948' : '#8f5637';
  px(ctx, r.x + 3, r.y + 3, r.w, r.h, '#2e2126'); // shadow on the wall and sideboard
  px(ctx, r.x, r.y, r.w, r.h, body);
  grain(ctx, r.x, r.y, r.w, r.h, dark, light, green ? 13 : 9);
  px(ctx, r.x, r.y, r.w, 3, lid); hline(ctx, r.x, r.y, r.w, green ? P.lime : '#b8744a');
  px(ctx, r.x, r.y, 2, r.h, light); px(ctx, r.x + r.w - 2, r.y, 2, r.h, dark);
  px(ctx, r.x, r.y + r.h - 3, r.w, 3, '#3e2119'); // plinth strip on the floor
  // lid plate: one pip per box, the one on show lit
  if (pages > 1) {
    const n = Math.min(pages, 8), pw = n * 4 + 3;
    const px0 = r.x + ((r.w - pw) / 2 | 0);
    px(ctx, px0, r.y + 4, pw, 6, P.cream);
    for (let i = 0; i < n; i++) px(ctx, px0 + 2 + i * 4, r.y + 6, 2, 2, i === page ? P.rust : P.slate);
  }
  const cols = 2, cw = ((r.w - 9) / cols) | 0, rh = 22;
  const rects = [];
  for (let slot = 0; slot < BOX_DRAWERS; slot++) {
    const col = slot % cols, row = (slot / cols) | 0;
    const x = r.x + 3 + col * (cw + 3), y = r.y + 13 + row * (rh + 3);
    const d = drawers[page * BOX_DRAWERS + slot];
    const out = openSlot === slot ? Math.round(openAmt * 14) : 0;
    drawDrawerFront(ctx, x, y, cw, rh, out, d ? d.number : null);
    if (d) rects.push({ index: d.index, x, y, w: cw, h: rh });
  }
  if (highlight) outline(ctx, r.x - 2, r.y - 2, r.w + 4, r.h + 4, P.yellow);
  return rects;
}

function drawDrawerFront(ctx, x, y, w, h, out, number) {
  if (out > 0) { // drawer body sliding out toward the viewer: draw it lower and wider
    px(ctx, x - 1, y + 2, w + 2, out, P.darkumber);
    px(ctx, x, y + out - 1, w, 2, P.cream); // a row of card tops
    for (let i = 2; i < w - 2; i += 3) px(ctx, x + i, y + out - 3, 2, 2, P.white);
  }
  const yy = y + out;
  px(ctx, x, yy, w, h, '#8f5637');
  grain(ctx, x, yy, w, h, '#7c4830', '#a6673f', x + y);
  hline(ctx, x, yy, w, '#b8744a');             // bevel: lit top edge
  px(ctx, x, yy + h - 1, w, 1, '#4a2a1c');      // and a shadowed bottom
  px(ctx, x, yy, 1, h, '#a6673f'); px(ctx, x + w - 1, yy, 1, h, '#5c3223');
  // number plate at the left of the face (none on an empty drawer), the pull at the right
  const py0 = yy + ((h / 2) | 0) - 3;
  if (number != null) {
    const pw = digitsWidth(number) + 4;
    px(ctx, x + 3, py0, pw, 7, P.cream);
    drawDigits(ctx, x + 5, py0 + 1, number, P.night);
  }
  const hx = x + w - 10;
  px(ctx, hx, py0 + 1, 6, 4, P.gold);
  px(ctx, hx + 1, py0 + 1, 4, 1, P.yellow);   // glint
  px(ctx, hx + 1, py0 + 4, 4, 1, P.amber);
}

// Precomputed lamp glow: quantised radial falloff with a 2x2 Bayer dither so
// it reads as pixel art rather than a gradient.
export function makeGlow(radius) {
  const size = radius * 2;
  const cv = document.createElement('canvas');
  cv.width = size; cv.height = size;
  const g = cv.getContext('2d');
  const img = g.createImageData(size, size);
  const bayer = [[0, 2], [3, 1]];
  for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) {
    const dx = x - radius, dy = (y - radius) * 1.4;
    const dist = Math.sqrt(dx * dx + dy * dy) / radius;
    let a = Math.max(0, 1 - dist);
    a = a * a * 4; // levels 0..4
    const level = Math.floor(a), frac = a - level;
    const on = level + (frac > bayer[y & 1][x & 1] / 4 ? 1 : 0);
    const alpha = Math.min(255, on * 40);
    const i = (y * size + x) * 4;
    img.data[i] = 254; img.data[i + 1] = 190; img.data[i + 2] = 90; img.data[i + 3] = alpha;
  }
  g.putImageData(img, 0, 0);
  return cv;
}

// Dithered vignette so the corners fall into shadow, the way a lamp-lit
// room reads. Precomputed once.
export function makeVignette(w, h) {
  const cv = document.createElement('canvas');
  cv.width = w; cv.height = h;
  const g = cv.getContext('2d');
  const img = g.createImageData(w, h);
  const bayer = [[0, 2], [3, 1]];
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
    const d = Math.min(x, w - x, y * 1.3, (h - y) * 1.3) / 70;
    let a = Math.max(0, 1 - Math.min(1, d));
    a = a * a * 4;
    const level = Math.floor(a), frac = a - level;
    const on = level + (frac > bayer[y & 1][x & 1] / 4 ? 1 : 0);
    const i = (y * w + x) * 4;
    img.data[i] = 24; img.data[i + 1] = 20; img.data[i + 2] = 37; img.data[i + 3] = Math.min(255, on * 48);
  }
  g.putImageData(img, 0, 0);
  return cv;
}

export function drawFlyingCard(ctx, x, y) {
  px(ctx, x + 2, y + 2, 16, 12, P.night);
  px(ctx, x, y, 16, 12, P.white);
  hline(ctx, x + 2, y + 3, 10, P.rose); hline(ctx, x + 2, y + 6, 11, P.fog); hline(ctx, x + 2, y + 8, 8, P.fog);
}
