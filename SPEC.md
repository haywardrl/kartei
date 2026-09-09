# Kartei — specification

Current as of 9 September 2026.

---

## 1. What it is

A desktop application that presents a personal note archive as a physical
room: a slip box of card drawers, a desk, and a corkboard. Notes are index
cards. Each new card is written *behind* an existing card, which builds a tree.
Cards also link to each other freely, which builds a graph. The tree gives the
archive a physical order; the links give it meaning.

The one-sentence pitch: **a real local-markdown zettelkasten inside a cosy room
where filing is the interaction.** Every cosy-productivity app on Steam treats
notes as a memo widget; every card-based note app is a flat canvas with no
place. Nobody combines them.

### Principles

1. **The app authors every file.** It creates notes with names it controls and
   never has to modify a file it did not write. Create-first, not import-first.
2. **Plain markdown in a folder the user picks.** Readable and editable in any
   other tool. If the app is uninstalled, nothing is lost.
3. **Filing is the point.** Placing a card forces a decision about what it
   relates to. That decision is the structure.
4. **The engine is headless.** The room is one possible view of it and could be
   replaced without touching the engine.
5. **The notes are never game content.** No quest cards, no pre-written
   archive. The box holds only what the user wrote.

### Non-goals for v1

Importing an existing vault; sync, accounts, or any server; rich text or
WYSIWYG; graph visualisation; plugins; mobile. Themes are deferred, but the
room is defined as data (layout plus sprite layers) so they stay cheap.

---

## 2. Decisions made and why

| Decision | Choice | Why |
|---|---|---|
| Engine | **Go core**, headless, tested, run as a helper process. | Every flow lives here; the room is a client of its API. |
| Room | **Wails**: the browser room in `ui/` inside a native window, engine bound in-process. Decided 9 September 2026 after trying both. | Godot was tried and reverted the same day: the art is the hard part, not the engine, and Wails keeps the editor, CSS iteration and nothing new to learn. The room's light and motion stay hand-drawn on a canvas. |
| Steam | **Deferred.** Ship on itch.io and direct download first. | Cosy-productivity sells on Steam (Spirit City: ~9,000 reviews, est. $4.4M gross), but those are broad "study with me" timers. A zettelkasten is a niche inside that niche. Decide after the picture test. |
| Art | Code-drawn placeholder art now; asset pack + hand-drawn hero objects later. | See §9. |
| Terminal UI | **Dropped.** | It cannot test a spatial filing gesture. The grey-box room replaced it. |

---

## 3. Architecture

```
  cmd/kartei         CLI: open vault, serve room, seed demo
  ui/                 the room (Canvas 2D) + paper panel (DOM), plain ES modules, go:embed
  internal/server     localhost JSON API + server-sent events
  internal/slipbox    headless engine — never imports anything above it
  vault/*.md + vault/.kartei/state.json
```

The engine exposes a single `Vault` type. The server holds one and calls
methods on it. The UI never touches the filesystem. `internal/slipbox` has no
knowledge of HTTP, HTML or any room.

Dependencies: `gopkg.in/yaml.v3`, `github.com/fsnotify/fsnotify`. Standard
library otherwise. No database, no network calls, no bundler.

---

## 4. Data model

### Identity

An ID is a creation timestamp in Denote format, UTC, second precision. The
filename is the ID plus a slug from the title:

```
20260908T142530--systems-fail-under-load.md
```

The ID is the identity and appears nowhere but the filename. The slug is
cosmetic; renaming it outside the app is harmless. Collisions increment by one
second. A file without a valid ID prefix is a **guest**: readable, linkable,
listed in the Unsorted drawer, never placed, never rewritten. Adoption is a
later explicit action.

### Note file

```markdown
---
title: Systems fail under load
created: 2026-09-08T14:25:30Z
tags: [systems, philosophy]
---

Body text with [[20260201T101500|links to other cards]].
```

Only `title`, `created`, `tags` are written. Unknown keys are preserved on
rewrite. **Never write an `id:` field.** No structural data in frontmatter.

### Sidecar `.kartei/state.json`

Inside the vault so it syncs and moves with the notes. **Store the exception,
derive the default:** only deliberately placed cards appear. A note absent from
`desk` and every board is in a drawer by rule.

**A card is in the box only when it has a `tree` entry** (version 2). An entry
with an empty parent is a root. A card with no entry is **unfiled**: written,
but without an address, and not in any drawer. Version 1 sidecars treated
"no entry" as root; they are migrated on load by giving every untracked card an
explicit root entry, so nothing moves.

**Boxes.** `settings.boxes` lists the slip boxes, main first; the default is
`main` (Notes, no prefix) and `lit` (Literature, prefix `L`). A root's tree
entry names its box; children inherit it. Addresses carry the prefix, drawers
are derived per box, and links cross boxes freely.

**The contents file.** `_index.md` in the vault root mirrors the tree, the
unfiled cards and the register in plain markdown after every structural
change. The app writes it and never reads it, except when the sidecar has no
tree at all and cards exist: then the tree is rebuilt from it.

**The desk is the inbox.** It holds exactly the unfiled cards; `desk` stores
only positions for the ones that have been dragged (membership is derived).
Filing a card drops its position. Filed cards you want to work with go on a
board.

```json
{
  "version": 2,
  "desk":     [{ "id": "20260315T142530", "x": 63, "y": 12 }],
  "boards":   [{ "id": "b_novel", "name": "Novel", "created": "…", "cards": [{ "id": "…", "x": 300, "y": 180 }] }],
  "tree":     { "20260201T101500": { "parent": "", "order": 1 },
                "20260315T142530": { "parent": "20260201T101500", "order": 2 } },
  "register": [{ "term": "feedback loops", "targets": ["…"], "note": "" }],
  "settings": { "drawerRule": "address-range", "drawerSize": 60, "deskCapacity": 12 },
  "prefs":    { "lamp": true, "camera": 60, "lastDrawer": 1, "activeBoard": "b_novel", "sound": false }
}
```

`prefs` is the one addition to the original schema: harmless UI traces so the
room remembers how it was left. The engine never reads them.

Board membership is a reference, not a move. A card may sit on several boards
and still lives in its drawer.

---

## 5. Algorithms (all implemented and table-tested in `internal/slipbox`)

- **Slug:** lowercase ASCII, accents folded, non-alphanumerics collapsed to one
  hyphen, 60 characters cut at a hyphen, empty becomes `untitled`.
- **Frontmatter:** absent is fine (title from slug, created from ID). A YAML
  error marks the note **damaged**: shown as-is, never rewritten.
- **Wikilinks:** `[[target]]` and `[[target|alias]]`, skipping fenced blocks
  and inline code. Resolution order: ID, slug, title, then current address
  (Luhmann linked by address; it is accepted for hand-typed links but the
  editor writes IDs because addresses change on refile). Ambiguity picks the
  oldest and warns. Broken links are kept and displayed, never dropped.
- **Address:** derived at load, never stored. Roots `1, 2, 3`; depth alternates
  numbers and letters (`1a`, `1a1`, `1a1a`); beyond `z` is bijective base-26.
  Siblings by `order`, then ID. Natural sort (`1a9` before `1a10`). Cycles are
  broken by promoting the lowest ID to a root, with a warning. An entry whose
  parent is missing makes the note a root.
- **Drawers:** `settings.drawerRule` chooses. `branch` (default): one drawer
  per root branch, in address order, numbered on the front and titled by the
  root; a branch past `drawerSize` spills into consecutive parts.
  `address-range`: the whole sequence chunked into equal drawers, Luhmann's
  non-topical boxes, for those who want them. Guests and damaged notes go to
  a final Unsorted drawer. Drawer identity is positional and never persisted.
- **Filing:** `File(id, parent, after)` puts a card behind a parent (empty for
  the root level) at a chosen position: after a named sibling, first, or last.
  Sibling orders are renumbered 1..n on both the new and the old parent, so
  they never drift. Filing a filed card moves its whole branch. You cannot
  file behind an unfiled card, or behind your own descendant.
- **Unfiled cards:** created from the pad or in another editor. They lie on
  the desk until filed or trashed; past `deskCapacity` the extras show as a
  pile. They are never rediscovered and have no address.
- **Safety:** every write goes through `WriteFileAtomic(root, path)`, which
  refuses any target outside the vault. `UpdateNote` compares the file's
  modification time and size with what the engine last read, and an editor
  passes the version it opened (`UpdateNoteSeen`); on mismatch nothing is
  written and a `ConflictError` carries the disk version so the user chooses.
  The first save of a session rotates `state.json` into `.kartei/backups/`
  (five kept); a sidecar that will not parse is set aside as
  `state.json.broken-<ts>` and the newest usable backup takes its place, with
  a warning.
- **Delete:** move to `.kartei/trash/` (suffixed on collision), drop
  placements and register entries, and slide the children into the deleted
  card's slot among its siblings so the sequence reads the same with one card
  gone.
- **Register:** user-curated terms pointing at cards, with an optional note in
  the user's words. The engine never adds a term itself.
- **Atomic write:** temp file in the same directory, fsync, rename. Never
  truncate in place.
- **Cleanup on load:** drop placements and tree entries that no longer resolve,
  unless more than 20% of referenced IDs are missing in a vault of more than 10
  notes. Then flag the vault incomplete, touch nothing, refuse to save.
- **Save:** note file first, then sidecar. Sidecar on a 500ms debounce and on
  close.
- **Watcher:** fsnotify on the vault root, 300ms debounce, own writes ignored
  for 2s, an empty read is never a deletion, a rename that keeps the ID keeps
  every placement. One changed file reparses one file.
- **Stage** (derived): drawer count 1 → box, 2–3 → double, 4–9 → cabinet, 10+
  → wall. Informational; the room draws boxes of six drawers instead (§7).
- **Rediscover** (derived): one card a day, deterministic, chosen from cards
  not on the desk or any board. Empty if fewer than three notes.

Measured: opening a 5,000-note vault takes about 160 ms on an M-series laptop
against a 2 s budget.

---

## 6. Engine API (`internal/slipbox.Vault`)

```
Open, Create, Root, Notes, Note, Warnings, Incomplete, Settings, SetSettings
Boxes, BoxOf, Address, Filed, Unfiled, Children, Parent, Roots(box), Backlinks, CrossRefs, Drawers, Stage, Rediscover
NewNote(title, body, parent), UpdateNote, UpdateNoteSeen, Overwrite, DeleteNote
File(id, parent, after), FileAsRoot(id, box, after)
Desk (derived), DeskMove(id, x, y)
Register, RegisterAdd(term, id, note), RegisterRemove(term, id)
Desk, DeskAdd(id, x, y), DeskRemove
BoardsList, NewBoard, RenameBoard, DeleteBoard, BoardPin(board, id, x, y), BoardUnpin
Prefs, SetPrefs, Search, Save, Close, Watch, Subscribe
```

Sentinel errors: `ErrNotFound`, `ErrGuestNote`, `ErrDamagedNote`,
`ErrVaultIncomplete`, `ErrCycle`, `ErrParentUnfiled`, `ErrNotSibling`,
`ErrConflict` (as `ConflictError`), `ErrOutsideVault`, `ErrNoSuchBox`. No panics
in the engine. The package is laid out for reading: `doc.go` explains the
model, `vault.go` opens and reads, `write.go` creates, edits and files,
`surfaces.go` holds desk, boards, register and persistence, `index.go` the
contents file.

### HTTP surface (`internal/server`)

`GET /api/vault` (whole model), `GET/POST/PUT/DELETE /api/note[/{id}]`,
`POST /api/note/{id}/file` (parent, after: sibling ID, `^` first, empty last;
box for a new root),
`PUT /api/desk/{id}` (position), `GET/POST/DELETE /api/register`,
`POST/PUT/DELETE /api/board[/{bid}]`, `POST/DELETE /api/board/{bid}/pin/{id}`,
`PUT /api/prefs`, `PUT /api/settings`, `GET /api/search?q=`,
`GET /api/events` (SSE `changed` with IDs). Errors map: not found 404, guest or
damaged 403, cycle / incomplete / conflict 409 (a conflict carries the disk
version). Every request needs the per-launch token in `X-Kartei-Token` (or
`?token=` for the event stream), and cross-origin requests are refused; this
exists only because the room runs in a browser and goes away with a native
shell.

---

## 7. The room

Base scene 560×270, viewport 480 wide, scaled by whole numbers with
`image-rendering: pixelated`. Wider windows see more of the room; narrower ones
pan. Text is never pixel art: the panel is DOM.

| Surface | Meaning | Rules |
|---|---|---|
| **Slip box** | The archive | A wooden card-index box with six drawer fronts, two across and three down, standing against the wall right of the desk. Each front carries its drawer number and nothing else; click it and it slides open. Past six drawers the archive is more boxes: click the box body to turn to the next, and pips on the lid say which of how many is showing. Opening a drawer from the panel turns to its box. |
| **Literature box** | The second box | A green box beside the first, same fronts, same pages. Sources go here as roots (`L1`), excerpts behind them. |
| **Desk** | The inbox | A tray holding the unfiled cards as one stack (red tab: a card slipped out today; orange pip: past the cap), a stack of blank cards, the register, the lamp. Click the desk or the tray for the top-down view, where the cards are laid out in full; drag them where you like and positions persist. |
| **Blank cards** | New card | A fresh card lands on the desk unfiled. No address until it is filed. |
| **Filing** | The decision | Press `f`. Open a drawer (in the panel or on the box), then drop the card *behind* another (a branch off it) or into the slot *after* one (next in its sequence). The address appears, the drawer gains a card, the desk clears. "Start a new branch" in either box is offered last. |
| **Contents ledger** | The index | On the wall shelf above the desk. Opens the list of every drawer in every box with its number and title: the lookup for which drawer to pick. |
| **Register** | Your index | The green ledger on the desk. Terms in your own words, then the drawers and addresses where they live. Enter a card under a term with `r`. |
| **Slipped card** | Rediscovery | One filed card a day surfaces in the desk view, white with a red tab; the tray shows the tab in the room. "Put it away" hides it until tomorrow. Not counted against the cap. |
| **Corkboard** | Working sets | Full screen, the room dimmed behind. Whole cards, drag to arrange, red thread between pinned cards that link to each other. One board shows in the room at a time. |
| **Lamp** | Cosiness | Toggles and is remembered. Night (by the system clock) darkens the room and the lamp pools light. |
| **Panel** | Text | Every view carries a back button that retraces the trail (drawers, cards, the register); editing and filing are steps, not places. |

**Two ways in.** Write *behind* a pulled card and it is filed at once, because
the choice was made when you pulled the card. Write from the *pad* and it is
unfiled on the desk until you file it. Either way, every card in the box got
there by a decision about what it relates to.

**Cross-references vs. descendants** are never merged: "mentions this" and
"grew from this" look different.

### Game loops

1. **Growth.** Past six drawers the slip box becomes a stack of boxes, shown
   as pips on the lid. Free, because drawers are derived. Later: objects and
   themes unlock by cards filed and days active, never by time spent.
2. **Rediscovery.** The box surfaces an old card. This is the real value of a
   zettelkasten and no app does it well. Cheap; high retention value.
3. **Gentle maintenance.** The desk cap, and later cards yellowing if left out.

### Cosiness, in order of payoff

One warm light in a dark room; sound (synthesised now: drawer roll, riffle,
pin, lamp click; samples later); traces of use (desk layout, camera, lamp and
last drawer persist); deliberate imperfection (the slipped card lies askew);
time of day.

---

## 8. External edits

The vault is a plain folder and people will edit notes in other tools. The
watcher reflects changes live over SSE: edit in Neovim, watch the card update
in the room. Debounced, own writes ignored, phantom sync events tolerated.

---

## 9. Art pipeline

**Now:** everything is drawn procedurally in `ui/art.js` from a locked
32-colour palette (Endesga 32), as named layers: panelled wall and floor, a
painting, the wall shelf with plant and contents ledger, corkboard with pins,
desk with inbox tray, blank cards and register, two slip boxes with drawer
animation, lamp, dithered lamp glow, darkness overlay by time of day. Each
layer is one function that a PNG could replace, and every rectangle lives in
the `L` table at the top of the file, which doubles as the hit regions.

**Next:** buy an interior tileset (LimeZu Modern Interiors / Modern Office on
itch.io, commercial licence with credit) for the room base. Hand-draw only the
three hero objects in Aseprite: the card, the drawer, the corkboard. Use Retro
Diffusion or PixelLab for drafts and clean them by hand. Aseprite exports a
spritesheet plus JSON; read that rather than hardcoding offsets. Commission a
single hero scene only if the picture test says go.

**Picture test (before committing a year):** one still of the room posted to
r/Zettelkasten, r/ObsidianMD, r/cozygames and Bluesky as "thinking about
building this". If a picture of the idea gets shrugs, the app will too.

---

## 10. Milestones

| # | Milestone | Status |
|---|---|---|
| 1 | Identity and notes: slugs, filenames, frontmatter round-trip, atomic writes | **done**, tested |
| 2 | Links and index: extraction, resolution, backlinks, warnings | **done**, tested |
| 3 | Tree, addresses, drawers, sidecar, cycle/orphan handling, mass-loss guard | **done**, tested |
| 4 | Vault API, search, delete-to-trash with reparenting, 5,000-note benchmark | **done**, tested |
| 5 | Localhost server, SSE, demo seed | **done**, tested |
| 6 | Grey-box room: drawers, pulled card with stack, desk with cap, boards, search | **done**, verified in a browser |
| 7 | Writing: editor, write-behind, new root, link autocomplete, edit, refile, trash | **done** |
| 8 | External edits reflected live | **done**, tested |
| 9 | Procedural art pass: palette, layers, lamp glow, time of day, tweens, sound | **done** (placeholder quality) |
| 9a | Filing as a gesture: unfiled state, inbox tray, positional filing, sibling order | **done**, tested |
| 9b | Register as a ledger on the desk | **done**, tested |
| 9c | Full-screen corkboard with whole cards and threads | **done** |
| 9d | Desk as inbox, drawers as numbered clickable branches, register by drawer, empty-list bug fixed | **done**, tested |
| 9e | Safety basics: launch token, path guard, sidecar backups and recovery, conflict detection | **done**, tested |
| 9f | Engine restructured for reading; boxes (notes + literature); `_index.md` contents with tree recovery; drawer labels and contents card in the room | **done**, tested |
| 10 | Wails app: native window around the browser room, engine bound in-process | **done** |
| 10a | Watcher ignores `_index.md` (it had been ingested as a guest card linking to everything); guest adoption | **done**, tested |
| 11 | Godot bootstrap and grey-box room | built, then **removed**; the Godot path is documented in `docs/UI-GUIDE.md` §8 if ever wanted |
| 12 | Index-card typography; top-down desk view; art pass: simpler desk, two paged slip boxes, painting and wall shelf, no window, candle or rug | **done** |
| 12a | Panel back button; number-only drawer fronts; "put it away" for the slipped card; drawer chooser reachable from a drawer; stable room scale when the panel opens | **done** |
| 13 | CodeMirror editor with markdown highlighting (needs a bundler) | next |
| 14 | Fuzzy search; the trail (a second pulled card lies beside the first) | later |
| 15 | Asset-pack art, hand-drawn hero objects, recorded sound | after the picture test |
| 16 | Growth unlocks and themes | after the picture test |
| 17 | Import tool: adopt an existing folder, seed a tree, let the user correct it | last |

---

## 11. Still to decide

- The desk cap is now visual (a pile past twelve). Is that enough pressure?
- Should writing behind a card also be a two-step (write, then file), for
  consistency with the pad? Current answer: no, pulling the card was the choice.
- Deferred trust feature, next in line: per-card history with restore.
- Drawers as branches nudge toward a root per subject. Watch whether users
  create many roots; the address-range rule is the fallback.
- Does pulling a second card replace the first in the panel (current) or lay
  beside it as a trail across the desk?
- Default `drawerSize` beyond 60. Real use will take months to fill a
  drawer, so the spill rule is untested in anger.
- What does a drawer look like at 800 cards, and can a card be found in under
  ten seconds?

---

## 12. Glossary

| Term | Meaning |
|---|---|
| **Card** | A note, as displayed in the room |
| **Address** | Derived Luhmann-style position, e.g. `1a3b`. Never stored |
| **Parent** | The card this one was filed behind. Stored in the sidecar |
| **Link** | A `[[wikilink]]` in the body. Written by the user |
| **Cross-reference** | A linked card that is not a descendant |
| **Drawer** | A chunk of the address sequence. Not a category |
| **Desk** | Capped set of in-flight cards |
| **Board** | A corkboard: a named working set. Membership is a reference |
| **Register** | User-written index of terms to cards |
| **Guest** | A file in the vault the app did not create |
| **Damaged** | A note whose frontmatter failed to parse. Shown, never rewritten |
| **Stage** | The engine's size class for the main box (box, double, cabinet, wall); the room shows boxes of six drawers instead |
| **Sidecar** | `.kartei/state.json`: all structure, placement and UI traces |
