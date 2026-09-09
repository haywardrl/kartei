// Package server exposes a Vault over a localhost JSON API and serves the
// room. It is the only place the engine and the UI meet.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/haywardrl/kartei/internal/model"
	"github.com/haywardrl/kartei/internal/slipbox"
)

// Server wraps a vault. All handlers are stateless beyond it.
type Server struct {
	v     *slipbox.Vault
	mux   *http.ServeMux
	token string
	index []byte
}

// New builds the handler. ui is the embedded room; pass nil to serve API only.
// token, when set, must accompany every API request: the room runs in a
// browser, and without it any web page could post to the local port. A
// native shell with no port needs none of this.
func New(v *slipbox.Vault, ui fs.FS, token string) *Server {
	s := &Server{v: v, mux: http.NewServeMux(), token: token}
	m := s.mux
	m.HandleFunc("GET /api/vault", s.getVault)
	m.HandleFunc("GET /api/note/{id}", s.getNote)
	m.HandleFunc("POST /api/note", s.postNote)
	m.HandleFunc("PUT /api/note/{id}", s.putNote)
	m.HandleFunc("DELETE /api/note/{id}", s.deleteNote)
	m.HandleFunc("POST /api/note/{id}/file", s.fileNote)
	m.HandleFunc("POST /api/note/{id}/adopt", s.adoptNote)
	m.HandleFunc("GET /api/register", s.getRegister)
	m.HandleFunc("POST /api/register", s.postRegister)
	m.HandleFunc("DELETE /api/register", s.deleteRegister)
	m.HandleFunc("PUT /api/desk/{id}", s.deskMove)
	m.HandleFunc("POST /api/board", s.postBoard)
	m.HandleFunc("PUT /api/board/{bid}", s.putBoard)
	m.HandleFunc("DELETE /api/board/{bid}", s.deleteBoard)
	m.HandleFunc("POST /api/board/{bid}/pin/{id}", s.pin)
	m.HandleFunc("DELETE /api/board/{bid}/pin/{id}", s.unpin)
	m.HandleFunc("PUT /api/prefs", s.putPrefs)
	m.HandleFunc("PUT /api/settings", s.putSettings)
	m.HandleFunc("GET /api/search", s.search)
	m.HandleFunc("GET /api/events", s.events)
	if ui != nil {
		if index, err := fs.ReadFile(ui, "index.html"); err == nil {
			s.index = []byte(strings.Replace(string(index), "<!--token-->", `<meta name="kartei-token" content="`+token+`">`, 1))
		}
		m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(s.index)
		})
		m.Handle("GET /", http.FileServerFS(ui))
	}
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if strings.HasPrefix(r.URL.Path, "/api/") && !s.authorised(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or wrong token"})
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin requests are refused"})
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) authorised(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	got := r.Header.Get("X-Kartei-Token")
	if got == "" {
		got = r.URL.Query().Get("token") // EventSource cannot set headers
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) == 1
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, val any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(val)
}

func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, slipbox.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, slipbox.ErrCycle), errors.Is(err, slipbox.ErrVaultIncomplete),
		errors.Is(err, slipbox.ErrParentUnfiled), errors.Is(err, slipbox.ErrNotSibling), errors.Is(err, slipbox.ErrOutsideVault),
		errors.Is(err, slipbox.ErrNoSuchBox):
		status = http.StatusConflict
	case errors.Is(err, slipbox.ErrGuestNote), errors.Is(err, slipbox.ErrDamagedNote):
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decode(r *http.Request, into any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(into); err != nil && !errors.Is(err, errEmpty) && err.Error() != "EOF" {
		return fmt.Errorf("bad request body: %w", err)
	}
	return nil
}

var errEmpty = errors.New("empty")

// --- handlers ---

func (s *Server) getVault(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, model.Build(s.v))
}

func (s *Server) getNote(w http.ResponseWriter, r *http.Request) {
	n, ok := s.v.Note(slipbox.ID(r.PathValue("id")))
	if !ok {
		writeErr(w, slipbox.ErrNotFound)
		return
	}
	writeJSON(w, 200, model.NewDetail(s.v, n))
}

func (s *Server) postNote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title  string     `json:"title"`
		Body   string     `json:"body"`
		Parent slipbox.ID `json:"parent"`
	}
	if err := decode(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	n, err := s.v.NewNote(in.Title, in.Body, in.Parent)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, model.NewDetail(s.v, n))
}

func (s *Server) putNote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string    `json:"title"`
		Body  string    `json:"body"`
		Seen  time.Time `json:"seen"` // modTime the editor loaded; zero to skip
	}
	if err := decode(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	id := slipbox.ID(r.PathValue("id"))
	var err error
	if r.URL.Query().Get("force") == "1" {
		err = s.v.Overwrite(id, in.Title, in.Body)
	} else {
		err = s.v.UpdateNoteSeen(id, in.Title, in.Body, in.Seen)
	}
	var conflict *slipbox.ConflictError
	if errors.As(err, &conflict) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": err.Error(), "conflict": true,
			"disk": map[string]string{"title": conflict.OnDisk.Title, "body": conflict.OnDisk.Body},
		})
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	n, _ := s.v.Note(id)
	writeJSON(w, 200, model.NewDetail(s.v, n))
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	if err := s.v.DeleteNote(slipbox.ID(r.PathValue("id"))); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// fileNote puts a card in the box at a chosen position.
func (s *Server) fileNote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Parent slipbox.ID `json:"parent"`
		Box    string     `json:"box"`   // for a new root: which box; "" = main
		After  slipbox.ID `json:"after"` // "" = last, "^" = first, else a sibling ID
	}
	if err := decode(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	id := slipbox.ID(r.PathValue("id"))
	var err error
	if in.Parent == "" {
		err = s.v.FileAsRoot(id, in.Box, in.After)
	} else {
		err = s.v.File(id, in.Parent, in.After)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	n, _ := s.v.Note(id)
	writeJSON(w, 200, model.NewDetail(s.v, n))
}

// adoptNote turns a guest file into a card.
func (s *Server) adoptNote(w http.ResponseWriter, r *http.Request) {
	n, err := s.v.Adopt(slipbox.ID(r.PathValue("id")))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, model.NewDetail(s.v, n))
}

func (s *Server) getRegister(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.v.Register())
}

func (s *Server) postRegister(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Term string     `json:"term"`
		ID   slipbox.ID `json:"id"`
		Note string     `json:"note"`
	}
	if err := decode(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.v.RegisterAdd(in.Term, in.ID, in.Note); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, s.v.Register())
}

func (s *Server) deleteRegister(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := s.v.RegisterRemove(q.Get("term"), slipbox.ID(q.Get("id"))); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, s.v.Register())
}

type xy struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (s *Server) deskMove(w http.ResponseWriter, r *http.Request) {
	var p xy
	if err := decode(r, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.v.DeskMove(slipbox.ID(r.PathValue("id")), p.X, p.Y); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, s.v.Desk())
}

func (s *Server) postBoard(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := decode(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	b, err := s.v.NewBoard(in.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, b)
}

func (s *Server) putBoard(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := decode(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.v.RenameBoard(r.PathValue("bid"), in.Name); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, s.v.BoardsList())
}

func (s *Server) deleteBoard(w http.ResponseWriter, r *http.Request) {
	if err := s.v.DeleteBoard(r.PathValue("bid")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, s.v.BoardsList())
}

func (s *Server) pin(w http.ResponseWriter, r *http.Request) {
	var p xy
	if err := decode(r, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.v.BoardPin(r.PathValue("bid"), slipbox.ID(r.PathValue("id")), p.X, p.Y); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, s.v.BoardsList())
}

func (s *Server) unpin(w http.ResponseWriter, r *http.Request) {
	if err := s.v.BoardUnpin(r.PathValue("bid"), slipbox.ID(r.PathValue("id"))); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, s.v.BoardsList())
}

func (s *Server) putPrefs(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	if err := decode(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.v.SetPrefs(in)
	writeJSON(w, 200, s.v.Prefs())
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	in := s.v.Settings()
	if err := decode(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.v.SetSettings(in)
	writeJSON(w, 200, s.v.Settings())
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, model.Summaries(s.v, s.v.Search(r.URL.Query().Get("q"))))
}

// events streams vault changes as server-sent events so the room can reflect
// edits made in any other editor.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	ch, stop := s.v.Subscribe()
	defer stop()
	fmt.Fprint(w, "event: hello\ndata: {}\n\n")
	flusher.Flush()
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, data)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
