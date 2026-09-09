package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/haywardrl/kartei/internal/model"
	"github.com/haywardrl/kartei/internal/slipbox"
)

// Bridge is what the front end calls. One method per action in
// docs/UI-GUIDE.md §5, same names, same argument order. Wails turns every
// exported method into window.go.main.Bridge.<Method>.
type Bridge struct {
	ctx  context.Context
	mu   sync.Mutex
	v    *slipbox.Vault
	stop func()
}

func (b *Bridge) vault() (*slipbox.Vault, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.v == nil {
		return nil, errors.New("no vault is open")
	}
	return b.v, nil
}

// attach makes v the current vault and forwards its change events to the
// front end as a "changed" event carrying the IDs.
func (b *Bridge) attach(v *slipbox.Vault) {
	b.mu.Lock()
	if b.stop != nil {
		b.stop()
	}
	if b.v != nil {
		b.v.Close()
	}
	b.v = v
	ch, stop := v.Subscribe()
	b.stop = stop
	b.mu.Unlock()
	_ = v.Watch()
	go func() {
		for ev := range ch {
			runtime.EventsEmit(b.ctx, "changed", ev.IDs)
		}
	}()
}

func (b *Bridge) closeVault() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stop != nil {
		b.stop()
		b.stop = nil
	}
	if b.v != nil {
		b.v.Close()
		b.v = nil
	}
}

func (b *Bridge) detailOf(v *slipbox.Vault, id string) (model.Detail, error) {
	n, ok := v.Note(slipbox.ID(id))
	if !ok {
		return model.Detail{}, slipbox.ErrNotFound
	}
	return model.NewDetail(v, n), nil
}

// --- reads ---

func (b *Bridge) Model() (model.Vault, error) {
	v, err := b.vault()
	if err != nil {
		return model.Vault{}, err
	}
	return model.Build(v), nil
}

func (b *Bridge) Note(id string) (model.Detail, error) {
	v, err := b.vault()
	if err != nil {
		return model.Detail{}, err
	}
	return b.detailOf(v, id)
}

func (b *Bridge) Search(q string) ([]model.Note, error) {
	v, err := b.vault()
	if err != nil {
		return nil, err
	}
	return model.Summaries(v, v.Search(q)), nil
}

// --- writes ---

func (b *Bridge) NewNote(title, body, parent string) (model.Detail, error) {
	v, err := b.vault()
	if err != nil {
		return model.Detail{}, err
	}
	n, err := v.NewNote(title, body, slipbox.ID(parent))
	if err != nil {
		return model.Detail{}, err
	}
	return model.NewDetail(v, n), nil
}

// UpdateNote saves an edit. seen is the modTime the editor loaded (RFC 3339,
// or empty); a conflict comes back as an error "conflict:<json>" so the
// front end can show both versions.
func (b *Bridge) UpdateNote(id, title, body, seen string, force bool) (model.Detail, error) {
	v, err := b.vault()
	if err != nil {
		return model.Detail{}, err
	}
	if force {
		err = v.Overwrite(slipbox.ID(id), title, body)
	} else {
		var t time.Time
		if seen != "" {
			t, _ = time.Parse(time.RFC3339Nano, seen)
		}
		err = v.UpdateNoteSeen(slipbox.ID(id), title, body, t)
	}
	var conflict *slipbox.ConflictError
	if errors.As(err, &conflict) {
		c := model.Conflict{Error: err.Error(), Conflict: true}
		c.Disk.Title, c.Disk.Body = conflict.OnDisk.Title, conflict.OnDisk.Body
		data, _ := json.Marshal(c)
		return model.Detail{}, fmt.Errorf("conflict:%s", data)
	}
	if err != nil {
		return model.Detail{}, err
	}
	return b.detailOf(v, id)
}

func (b *Bridge) DeleteNote(id string) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	return v.DeleteNote(slipbox.ID(id))
}

func (b *Bridge) Adopt(id string) (model.Detail, error) {
	v, err := b.vault()
	if err != nil {
		return model.Detail{}, err
	}
	n, err := v.Adopt(slipbox.ID(id))
	if err != nil {
		return model.Detail{}, err
	}
	return model.NewDetail(v, n), nil
}

// File puts a card in a box: behind parent, or as a new root of box when
// parent is empty; after positions it among its siblings ("" last, "^" first).
func (b *Bridge) File(id, parent, box, after string) (model.Detail, error) {
	v, err := b.vault()
	if err != nil {
		return model.Detail{}, err
	}
	if parent == "" {
		err = v.FileAsRoot(slipbox.ID(id), box, slipbox.ID(after))
	} else {
		err = v.File(slipbox.ID(id), slipbox.ID(parent), slipbox.ID(after))
	}
	if err != nil {
		return model.Detail{}, err
	}
	return b.detailOf(v, id)
}

func (b *Bridge) DeskMove(id string, x, y int) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	return v.DeskMove(slipbox.ID(id), x, y)
}

// --- boards ---

func (b *Bridge) NewBoard(name string) (slipbox.Board, error) {
	v, err := b.vault()
	if err != nil {
		return slipbox.Board{}, err
	}
	return v.NewBoard(name)
}

func (b *Bridge) RenameBoard(bid, name string) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	return v.RenameBoard(bid, name)
}

func (b *Bridge) DeleteBoard(bid string) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	return v.DeleteBoard(bid)
}

func (b *Bridge) Pin(bid, id string, x, y int) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	return v.BoardPin(bid, slipbox.ID(id), x, y)
}

func (b *Bridge) Unpin(bid, id string) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	return v.BoardUnpin(bid, slipbox.ID(id))
}

// --- register, prefs, settings ---

func (b *Bridge) RegisterAdd(term, id, note string) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	return v.RegisterAdd(term, slipbox.ID(id), note)
}

func (b *Bridge) RegisterRemove(term, id string) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	return v.RegisterRemove(term, slipbox.ID(id))
}

func (b *Bridge) SetPrefs(p map[string]any) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	v.SetPrefs(p)
	return nil
}

func (b *Bridge) SetSettings(s slipbox.Settings) error {
	v, err := b.vault()
	if err != nil {
		return err
	}
	v.SetSettings(s)
	return nil
}
