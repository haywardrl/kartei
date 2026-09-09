# Building a front end on the Kartei engine

This is the contract between the engine and any user interface. The bundled
browser room in `ui/` is one client of it and nothing more; a Godot scene, a
Wails window or a terminal could replace it without touching the engine.

Read `internal/slipbox/doc.go` first for the model in prose. This document is
about wiring.

## 1. Two ways to connect

**A. Over HTTP on localhost (any language, any engine).** Run the engine as a
process and talk JSON to it. This is how the browser room works today and how
a Godot front end would work.

```
go run ./cmd/kartei --vault ~/Kartei --no-open --port 8765 --token devtoken
```

It prints `kartei: room at http://127.0.0.1:8765` and `kartei: token devtoken`.
Without `--token` a random one is printed; a front end must send it on every
request as the `X-Kartei-Token` header (or `?token=` on the event stream).
Requests that carry an `Origin` header from another origin are refused. Both
exist only because a browser might be involved; they cost one line per request.

**B. In-process Go (Wails, or any Go shell).** Import `internal/slipbox`, call
`slipbox.Open(dir)` and use the `Vault` methods directly. No server, no token.
This is how the shipped app works (§9).

Everything below is written for A, with the Go method named beside each route
for B.

## 2. The shape of a session

```
open  ──▶  GET /api/vault  ──▶  draw the room from the model
                                      │
   user acts (click, drag, type)      │
        │                             ▼
        └──▶ POST/PUT/DELETE …  ──▶ engine writes files + sidecar
                                      │
             GET /api/events (SSE) ◀──┘  "changed" with the card IDs
                                      │
                                      └──▶ GET /api/vault again, redraw
```

The rule that keeps a client simple: **after any change, from you or from
another editor, reload the model and redraw.** The model is small (a few
hundred bytes per card, 5,000 cards in well under a second) and every
derived fact is in it, so the client holds no state of its own beyond what the
user is looking at. The browser room does exactly this and nothing cleverer.

## 3. The model: `GET /api/vault`  (Go: several read methods)

One object with everything a room needs to draw itself.

```jsonc
{
  "root": "/Users/you/Kartei",
  "notes": [                      // every card, guests last
    { "id": "20260909T081522", "title": "One idea per card", "slug": "one-idea-per-card",
      "path": "20260909T081522--one-idea-per-card.md",
      "address": "1a",            // "" for unfiled and guests
      "box": "main",              // "" for unfiled; "lit" for the literature box
      "filed": true,
      "parent": "20260909T081000",// "" for roots and unfiled
      "children": ["…"],          // in order: what is filed behind it
      "links": ["…"],             // resolved outgoing wikilinks
      "tags": [], "created": "2026-09-09T08:15:22Z",
      "guest": false, "damaged": false,
      "excerpt": "first line of the body" }
  ],
  "boxes":   [ { "id": "main", "name": "Notes", "prefix": "" }, { "id": "lit", "name": "Literature", "prefix": "L" } ],
  "drawers": [                    // main box first, then other boxes, then Unsorted
    { "index": 0, "box": "main", "number": 1, "label": "1 · Attention is the scarce input",
      "title": "Attention is the scarce input", "root": "…", "part": 1, "parts": 1,
      "first": "1", "last": "1c", "ids": ["…"], "unsorted": false }
  ],
  "desk":     [ { "id": "…", "x": -1, "y": -1 } ],   // exactly the unfiled cards; -1 = not placed yet
  "unfiled":  ["…"],
  "boards":   [ { "id": "b_essay", "name": "Essay", "created": "…", "cards": [ { "id": "…", "x": 80, "y": 60 } ] } ],
  "register": [ { "term": "attention", "targets": ["…"], "note": "the scarce input" } ],
  "settings": { "drawerRule": "branch", "drawerSize": 60, "deskCapacity": 12, "boxes": [ … ] },
  "warnings": [ { "kind": "broken-link", "id": "…", "message": "[[x]] does not resolve" } ],
  "stage": "cabinet",             // box | double | cabinet | wall, from the main box's drawer count
  "rediscover": "…",              // one filed card for today, or ""
  "prefs": { "lamp": true, "camera": 60 },   // anything the UI stored; the engine ignores it
  "incomplete": false             // true = the folder looks half-synced; show a warning, expect saves to fail
}
```

What to draw from it:

| Room object | Source |
|---|---|
| Slip box and its drawer fronts | `drawers` where `box` is the first of `boxes`; the bundled room shows six per box and pages past that |
| Literature box | `drawers` where `box == "lit"` |
| Number on each drawer | `number`; the `title` belongs in a contents list, not on the front |
| Size class, if you want one | `stage`: box, double, cabinet, wall, from the main box's drawer count |
| Desk | `desk`, one card each; place unplaced ones yourself |
| Pile on the desk | count of `desk` beyond `settings.deskCapacity` |
| Corkboard pins | the active board's `cards` |
| Contents card | `drawers` grouped by `boxes` |
| Ledger | `register` |
| Slipped-out card | `rediscover` |
| Empty lists are always `[]`, never `null`. | |

## 4. One card: `GET /api/note/{id}`  (Go: `Note`, `Backlinks`, `CrossRefs`)

Everything in the summary above, plus:

```jsonc
{ "body": "markdown…", "modTime": "2026-09-09T08:15:22.1234Z",
  "links": [ { "raw": "[[20260909T081000|one idea]]", "target": "20260909T081000", "alias": "one idea", "resolved": "20260909T081000" } ],
  "backlinks": ["…"], "crossrefs": ["…"], "extra": { "source": "…" } }
```

Keep `modTime`: send it back when saving so the engine can tell if the card
changed under you (section 6).

Render `body` yourself. The bundled room turns paragraphs into `<p>` and
`[[…]]` into links using `links[].resolved`. In Godot, a `RichTextLabel` with
`[url]` tags does the same job.

## 5. Actions

Every write returns either the updated card, the updated list, or
`{ "error": "…" }` with a status you can branch on:

| Status | Meaning | Show |
|---|---|---|
| 404 | no such card, board or term | "that card is gone" |
| 403 | guest or damaged note | "this file was not written by the room" |
| 409 | cycle, unknown box, filing behind an unfiled card, vault incomplete, or a **conflict** | the message; for a conflict, section 6 |
| 401 | missing token | a bug in your client |

| Gesture | Route | Body | Go |
|---|---|---|---|
| Write a fresh card (it lands on the desk) | `POST /api/note` | `{title, body}` | `NewNote(title, body, "")` |
| Write behind a card (filed at once) | `POST /api/note` | `{title, body, parent}` | `NewNote(title, body, parent)` |
| Edit | `PUT /api/note/{id}` | `{title, body, seen: modTime}` | `UpdateNoteSeen` |
| Edit, overriding a conflict | `PUT /api/note/{id}?force=1` | same | `Overwrite` |
| Trash | `DELETE /api/note/{id}` | | `DeleteNote` |
| Adopt a guest file (give it an ID; it lands on the desk) | `POST /api/note/{id}/adopt` | | `Adopt` |
| **File behind a card** (a branch off it) | `POST /api/note/{id}/file` | `{parent: P}` | `File(id, P, "")` |
| **File after a card** (next in its line) | same | `{parent: P.parent, after: P}` | `File(id, P.parent, P)` |
| File first among siblings | same | `{parent: P, after: "^"}` | `File(id, P, "^")` |
| **Start a new branch** in a box | same | `{box: "main" or "lit"}` | `FileAsRoot(id, box, "")` |
| Move a desk card | `PUT /api/desk/{id}` | `{x, y}` | `DeskMove` |
| New / rename / delete board | `POST /api/board`, `PUT /api/board/{bid}`, `DELETE /api/board/{bid}` | `{name}` | `NewBoard`, `RenameBoard`, `DeleteBoard` |
| Pin / move a pin | `POST /api/board/{bid}/pin/{id}` | `{x, y}` | `BoardPin` |
| Unpin | `DELETE /api/board/{bid}/pin/{id}` | | `BoardUnpin` |
| Register: add a card under a term | `POST /api/register` | `{term, id, note}` | `RegisterAdd` |
| Register: remove a card, or a whole term with no id | `DELETE /api/register?term=…&id=…` | | `RegisterRemove` |
| Remember UI state (camera, lamp, last drawer) | `PUT /api/prefs` | any JSON object, merged | `SetPrefs` |
| Change rules (drawer rule, sizes, boxes) | `PUT /api/settings` | partial `settings` | `SetSettings` |
| Search titles then bodies | `GET /api/search?q=` | | `Search` |

Filing is the one gesture worth getting right. In the bundled room the user
opens a drawer, sees its cards in address order, and each card offers two
targets: **behind** (parent = that card) and **after** (parent = that card's
parent, after = that card). Both are one call. A drag-and-drop UI drops the
card on the same two targets. Refiling a filed card is the same call; its
whole branch moves.

## 6. Conflicts, and edits made elsewhere

The vault is a plain folder and people will edit it in other tools. Two
things follow.

**Live updates.** `GET /api/events` is a server-sent event stream. Each
`changed` event carries the IDs that changed, from any source. On it, reload
the model and, if a changed card is open, its detail. In a browser this is
`EventSource`. In Godot, either read the stream with `HTTPClient` in chunked
mode, or simply poll `GET /api/vault` every second or two; it is cheap.

**Conflicts.** When you save with `seen` set to the `modTime` you loaded, and
the file has changed since, the engine writes nothing and answers:

```json
{ "error": "the note changed on disk since it was opened", "conflict": true,
  "disk": { "title": "…", "body": "…" } }
```

Show both versions and offer: keep mine (`?force=1`), keep theirs (reload),
keep both (save theirs as a new card with `POST /api/note`). Never overwrite
silently; that is the one way a note tool loses a user for good.

## 7. What the UI must not do

- Touch the vault folder itself. Every write goes through the engine.
- Store structure anywhere but the sidecar via the engine. UI conveniences
  (camera, lamp, last drawer) go in `prefs`.
- Invent addresses, drawer membership or box membership. They are derived;
  take them from the model.
- Add cards the user did not write.

## 8. A Godot client, in outline

One autoload script, `Kartei.gd`, owns the connection:

```gdscript
extends Node
var base := "http://127.0.0.1:8765"
var token := "devtoken"          # or read it from the process's stdout
var model := {}
signal model_changed

func _ready() -> void:
    reload()
    var t := Timer.new(); t.wait_time = 1.5; t.autostart = true
    t.timeout.connect(reload); add_child(t)   # polling stands in for SSE

func _headers() -> PackedStringArray:
    return ["X-Kartei-Token: " + token, "Content-Type: application/json"]

func reload() -> void:
    _call("GET", "/api/vault", "", func(data): model = data; model_changed.emit())

func file_behind(id: String, parent: String) -> void:
    _call("POST", "/api/note/%s/file" % id, JSON.stringify({"parent": parent}), func(_d): reload())

func _call(method: String, path: String, body: String, on_done: Callable) -> void:
    var req := HTTPRequest.new(); add_child(req)
    req.request_completed.connect(func(_r, code, _h, bytes):
        var data = JSON.parse_string(bytes.get_string_from_utf8())
        if code >= 400: push_warning(data.get("error", "request failed"))
        else: on_done.call(data)
        req.queue_free())
    var m := {"GET": HTTPClient.METHOD_GET, "POST": HTTPClient.METHOD_POST, "PUT": HTTPClient.METHOD_PUT, "DELETE": HTTPClient.METHOD_DELETE}[method]
    req.request(base + path, _headers(), m, body)
```

The room scene listens to `model_changed` and rebuilds: one `Drawer` scene
instance per entry in `model.drawers` for the main box, positioned in a
cabinet node; one `DeskCard` per `model.desk` entry; a `Label` on each drawer
from `number`. Clicking a drawer emits the drawer index; the
panel (a `Control` overlay with `RichTextLabel`, `LineEdit`, `TextEdit`)
fetches `/api/note/{id}` when a card is opened. Ship the Go binary inside the
`.app` and launch it with `OS.create_process` at startup, passing `--token`.

## 9. The Wails client (built: `app/`)

`app/main.go` opens the window with `AssetServer{Assets: ui.FS}` and binds
`app/bridge.go`, a `Bridge` with one method per action in §5. A goroutine
forwards `v.Subscribe()` events with `runtime.EventsEmit(ctx, "changed", ids)`,
and the native menu emits `"menu"` events. The HTTP layer, the token and the
origin check disappear. `ui/api.js` detects `window.go.main.Bridge` and routes
every call to it, falling back to HTTP in a browser, so `app.js` never knows
which transport it is on. `KARTEI_VAULT` overrides the folder picker.

## 10. Testing a front end without the room

The engine has a demo vault. `go run ./cmd/kartei --demo --serve --no-open --token devtoken`
seeds 24 cards in a temp folder and serves the API. Every route is exercised
in `internal/server/server_test.go`, which is also the shortest complete
example of the request and response shapes.
