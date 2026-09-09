package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/haywardrl/kartei/internal/slipbox"
)

func call(t *testing.T, h http.Handler, method, path string, body any) (int, map[string]any, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := map[string]any{}
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out, rec.Body.Bytes()
}

func TestRoutesEndToEnd(t *testing.T) {
	v, err := slipbox.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	h := New(v, nil, "")

	code, root, _ := call(t, h, "POST", "/api/note", map[string]any{"title": "Root", "body": "first"})
	if code != 201 || root["address"] != "" || root["filed"] != false {
		t.Fatalf("a new card with no parent is unfiled: %d %v", code, root)
	}
	rootID := root["id"].(string)
	if code, out, _ := call(t, h, "POST", "/api/note/"+rootID+"/file", map[string]any{"parent": "", "after": ""}); code != 200 || out["address"] != "1" {
		t.Fatalf("file as root: %d %v", code, out)
	}
	code, child, _ := call(t, h, "POST", "/api/note", map[string]any{"title": "Child", "body": "see [[" + rootID + "|root]]", "parent": rootID})
	if code != 201 || child["address"] != "1a" {
		t.Fatalf("create child: %d %v", code, child)
	}
	childID := child["id"].(string)

	code, detail, _ := call(t, h, "GET", "/api/note/"+rootID, nil)
	if code != 200 || len(detail["backlinks"].([]any)) != 1 || len(detail["children"].([]any)) != 1 {
		t.Fatalf("detail: %d %v", code, detail)
	}
	if code, _, _ := call(t, h, "GET", "/api/note/nope", nil); code != 404 {
		t.Fatalf("missing note should 404, got %d", code)
	}

	if code, _, _ := call(t, h, "PUT", "/api/note/"+childID, map[string]any{"title": "Child renamed", "body": "new"}); code != 200 {
		t.Fatalf("update: %d", code)
	}
	if code, _, _ := call(t, h, "POST", "/api/note/"+rootID+"/file", map[string]any{"parent": childID}); code != 409 {
		t.Fatalf("cycle should 409, got %d", code)
	}
	_, lit, _ := call(t, h, "POST", "/api/note", map[string]any{"title": "A source", "body": "read it"})
	if code, out, _ := call(t, h, "POST", "/api/note/"+lit["id"].(string)+"/file", map[string]any{"box": "lit"}); code != 200 || out["address"] != "L1" || out["box"] != "lit" {
		t.Fatalf("file into the literature box: %d %v", code, out)
	}
	if code, _, _ := call(t, h, "POST", "/api/note/"+lit["id"].(string)+"/file", map[string]any{"box": "nope"}); code != 409 {
		t.Fatalf("unknown box should 409, got %d", code)
	}
	call(t, h, "DELETE", "/api/note/"+lit["id"].(string), nil)

	code, loose, _ := call(t, h, "POST", "/api/note", map[string]any{"title": "Loose", "body": "unfiled"})
	looseID := loose["id"].(string)
	_, model0, _ := call(t, h, "GET", "/api/vault", nil)
	if desk := model0["desk"].([]any); len(desk) != 1 || desk[0].(map[string]any)["id"] != looseID {
		t.Fatalf("the desk holds exactly the unfiled cards: %v", desk)
	}
	if code, _, _ := call(t, h, "PUT", "/api/desk/"+looseID, map[string]any{"x": 5, "y": 6}); code != 200 {
		t.Fatalf("desk move: %d", code)
	}
	if code, _, _ := call(t, h, "PUT", "/api/desk/"+rootID, map[string]any{"x": 5, "y": 6}); code != 404 {
		t.Fatalf("filed cards are not on the desk: %d", code)
	}
	if code, out, _ := call(t, h, "POST", "/api/note/"+looseID+"/file", map[string]any{"parent": rootID, "after": "^"}); code != 200 || out["address"] != "1a" {
		t.Fatalf("file first under root: %d %v", code, out)
	}
	_, model1, _ := call(t, h, "GET", "/api/vault", nil)
	if desk := model1["desk"].([]any); len(desk) != 0 {
		t.Fatalf("filing clears the desk: %v", desk)
	}
	if code, _, _ := call(t, h, "POST", "/api/register", map[string]any{"term": "feedback loops", "id": looseID, "note": "systems"}); code != 200 {
		t.Fatalf("register add: %d", code)
	}
	_, _, rawReg := call(t, h, "GET", "/api/register", nil)
	var reg []map[string]any
	json.Unmarshal(rawReg, &reg)
	if len(reg) != 1 || reg[0]["term"] != "feedback loops" || len(reg[0]["targets"].([]any)) != 1 {
		t.Fatalf("register: %s", rawReg)
	}
	if code, _, _ := call(t, h, "DELETE", "/api/register?term=feedback+loops&id="+looseID, nil); code != 200 {
		t.Fatalf("register remove: %d", code)
	}

	code, board, _ := call(t, h, "POST", "/api/board", map[string]any{"name": "Essay"})
	if code != 201 || board["id"] != "b_essay" {
		t.Fatalf("board: %d %v", code, board)
	}
	if code, _, _ := call(t, h, "POST", "/api/board/b_essay/pin/"+childID, map[string]any{"x": 10, "y": 20}); code != 200 {
		t.Fatalf("pin: %d", code)
	}
	if code, _, _ := call(t, h, "PUT", "/api/board/b_essay", map[string]any{"name": "Essay two"}); code != 200 {
		t.Fatalf("rename: %d", code)
	}
	if code, _, _ := call(t, h, "DELETE", "/api/board/b_essay/pin/"+childID, nil); code != 200 {
		t.Fatalf("unpin: %d", code)
	}
	if code, _, _ := call(t, h, "PUT", "/api/prefs", map[string]any{"lamp": false}); code != 200 {
		t.Fatalf("prefs: %d", code)
	}

	_, _, raw := call(t, h, "GET", "/api/search?q=renamed", nil)
	var hits []map[string]any
	json.Unmarshal(raw, &hits)
	if len(hits) != 1 || hits[0]["id"] != childID {
		t.Fatalf("search: %s", raw)
	}

	code, model, raw := call(t, h, "GET", "/api/vault", nil)
	if code != 200 || len(model["notes"].([]any)) != 3 || model["stage"] != "box" || model["prefs"].(map[string]any)["lamp"] != false || len(model["boxes"].([]any)) != 2 {
		t.Fatalf("vault model: %d %v", code, model)
	}
	for _, want := range []string{`"desk":[]`, `"targets":[]`, `"cards":[]`} {
		if !bytes.Contains(raw, []byte(want)) {
			t.Fatalf("empty lists must be [] not null; missing %s in %s", want, raw)
		}
	}
	if len(model["boards"].([]any)) != 1 || model["boards"].([]any)[0].(map[string]any)["name"] != "Essay two" {
		t.Fatalf("boards in model: %v", model["boards"])
	}

	if code, _, _ := call(t, h, "DELETE", "/api/note/"+rootID, nil); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	_, model, _ = call(t, h, "GET", "/api/vault", nil)
	addr := map[string]string{}
	for _, n := range model["notes"].([]any) {
		m := n.(map[string]any)
		addr[m["id"].(string)] = m["address"].(string)
	}
	if len(addr) != 2 || addr[looseID] != "1" || addr[childID] != "2" {
		t.Fatalf("children should become roots keeping their order after delete: %v", addr)
	}
	if code, _, _ := call(t, h, "DELETE", "/api/board/b_essay", nil); code != 200 {
		t.Fatalf("delete board: %d", code)
	}
}

func TestEmptyVaultHasNoNulls(t *testing.T) {
	v, _ := slipbox.Open(t.TempDir())
	defer v.Close()
	h := New(v, nil, "")
	_, _, raw := call(t, h, "GET", "/api/vault", nil)
	if bytes.Contains(raw, []byte("null")) {
		t.Fatalf("empty vault model contains null: %s", raw)
	}
	_, board, rawBoard := call(t, h, "POST", "/api/board", map[string]any{"name": "Fresh"})
	if !bytes.Contains(rawBoard, []byte(`"cards":[]`)) {
		t.Fatalf("new board must have an empty card list: %s %v", rawBoard, board)
	}
}

func TestTokenAndOrigin(t *testing.T) {
	v, _ := slipbox.Open(t.TempDir())
	defer v.Close()
	h := New(v, nil, "secret")
	req := httptest.NewRequest("GET", "/api/vault", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("no token should be 401, got %d", rec.Code)
	}
	req = httptest.NewRequest("GET", "/api/vault", nil)
	req.Header.Set("X-Kartei-Token", "secret")
	req.Header.Set("Origin", "http://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("foreign origin should be 403, got %d", rec.Code)
	}
	req = httptest.NewRequest("GET", "/api/vault?token=secret", nil)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("own origin with query token should pass, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestConflictReturnsDiskVersion(t *testing.T) {
	dir := t.TempDir()
	v, _ := slipbox.Open(dir)
	defer v.Close()
	h := New(v, nil, "")
	_, n, _ := call(t, h, "POST", "/api/note", map[string]any{"title": "Edited twice", "body": "first"})
	id := n["id"].(string)
	path := filepath.Join(dir, n["path"].(string))
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(path, []byte("---\ntitle: Edited twice\n---\n\nfrom vim\n"), 0o644)
	code, out, _ := call(t, h, "PUT", "/api/note/"+id, map[string]any{"title": "Edited twice", "body": "from the room"})
	if code != 409 || out["conflict"] != true || out["disk"].(map[string]any)["body"] != "from vim\n" {
		t.Fatalf("conflict: %d %v", code, out)
	}
	if code, _, _ := call(t, h, "PUT", "/api/note/"+id+"?force=1", map[string]any{"title": "Edited twice", "body": "from the room"}); code != 200 {
		t.Fatalf("force: %d", code)
	}
}
