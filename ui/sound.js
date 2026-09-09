// Synthesised room sounds. No sample files, so the binary stays self-contained;
// swap any of these for a recording later without touching callers.
let ctx = null;
let enabled = false;

function ac() {
  if (!ctx) ctx = new (window.AudioContext || window.webkitAudioContext)();
  if (ctx.state === 'suspended') ctx.resume();
  return ctx;
}

function noise(duration, { filter = 'lowpass', freq = 800, q = 1, gain = 0.3, attack = 0.005, sweep = null } = {}) {
  if (!enabled) return;
  const c = ac();
  const len = Math.floor(c.sampleRate * duration);
  const buf = c.createBuffer(1, len, c.sampleRate);
  const d = buf.getChannelData(0);
  for (let i = 0; i < len; i++) d[i] = Math.random() * 2 - 1;
  const src = c.createBufferSource();
  src.buffer = buf;
  const f = c.createBiquadFilter();
  f.type = filter; f.frequency.value = freq; f.Q.value = q;
  if (sweep) f.frequency.linearRampToValueAtTime(sweep, c.currentTime + duration);
  const g = c.createGain();
  g.gain.setValueAtTime(0, c.currentTime);
  g.gain.linearRampToValueAtTime(gain, c.currentTime + attack);
  g.gain.exponentialRampToValueAtTime(0.001, c.currentTime + duration);
  src.connect(f).connect(g).connect(c.destination);
  src.start();
}

function tone(freq, duration, { type = 'sine', gain = 0.15, slide = null } = {}) {
  if (!enabled) return;
  const c = ac();
  const o = c.createOscillator();
  o.type = type; o.frequency.value = freq;
  if (slide) o.frequency.exponentialRampToValueAtTime(slide, c.currentTime + duration);
  const g = c.createGain();
  g.gain.setValueAtTime(gain, c.currentTime);
  g.gain.exponentialRampToValueAtTime(0.001, c.currentTime + duration);
  o.connect(g).connect(c.destination);
  o.start(); o.stop(c.currentTime + duration);
}

export const sound = {
  get enabled() { return enabled; },
  set enabled(v) { enabled = v; if (v) ac(); },
  drawer() { noise(0.32, { freq: 300, sweep: 900, gain: 0.25, q: 0.7 }); setTimeout(() => noise(0.06, { freq: 1200, filter: 'bandpass', gain: 0.18 }), 260); },
  drawerClose() { noise(0.22, { freq: 900, sweep: 250, gain: 0.22 }); setTimeout(() => tone(140, 0.08, { type: 'triangle', gain: 0.12 }), 180); },
  paper() { noise(0.09, { filter: 'highpass', freq: 2500, gain: 0.12 }); },
  riffle() { for (let i = 0; i < 5; i++) setTimeout(() => noise(0.04, { filter: 'highpass', freq: 3000, gain: 0.08 }), i * 35); },
  pin() { tone(1400, 0.05, { type: 'square', gain: 0.06 }); noise(0.03, { filter: 'highpass', freq: 4000, gain: 0.1 }); },
  place() { noise(0.05, { freq: 600, gain: 0.15 }); },
  lamp() { tone(2200, 0.03, { type: 'square', gain: 0.05 }); },
  nope() { tone(220, 0.15, { type: 'triangle', gain: 0.1, slide: 160 }); },
  save() { tone(660, 0.08, { gain: 0.08 }); setTimeout(() => tone(880, 0.12, { gain: 0.08 }), 70); },
};
