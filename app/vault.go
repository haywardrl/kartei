package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/haywardrl/kartei/internal/slipbox"
)

// config remembers which folder is the vault, in the OS's app-data dir; it is
// the only thing the app writes outside the vault.
type config struct {
	Vault string `json:"vault"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Kartei", "config.json"), nil
}

func loadConfig() config {
	var c config
	if p, err := configPath(); err == nil {
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &c)
		}
	}
	return c
}

func saveConfig(c config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, data, 0o644)
}

// openVault opens the remembered vault, or asks for a folder on first run
// (defaulting to ~/Kartei if the dialog is dismissed).
func (b *Bridge) openVault() error {
	if dir := os.Getenv("KARTEI_VAULT"); dir != "" { // development and tests: skip the dialog and the config
		return b.useVault(dir)
	}
	c := loadConfig()
	if c.Vault == "" {
		c.Vault = b.chooseFolder("Choose a folder for your slip box (it will hold plain markdown files)")
		if c.Vault == "" {
			home, _ := os.UserHomeDir()
			c.Vault = filepath.Join(home, "Kartei")
		}
		if err := saveConfig(c); err != nil {
			return err
		}
	}
	return b.useVault(c.Vault)
}

func (b *Bridge) useVault(dir string) error {
	v, err := slipbox.Open(dir)
	if err != nil {
		return err
	}
	b.attach(v)
	runtime.WindowSetTitle(b.ctx, "Kartei — "+filepath.Base(dir))
	return nil
}

func (b *Bridge) chooseFolder(title string) string {
	home, _ := os.UserHomeDir()
	dir, err := runtime.OpenDirectoryDialog(b.ctx, runtime.OpenDialogOptions{Title: title, DefaultDirectory: home, CanCreateDirectories: true})
	if err != nil || dir == "" {
		return ""
	}
	return dir
}

// SwitchVault is the menu's "Open vault…": pick a folder, remember it, reload.
func (b *Bridge) SwitchVault() error {
	dir := b.chooseFolder("Open a slip box folder")
	if dir == "" {
		return errors.New("no folder chosen")
	}
	if err := b.useVault(dir); err != nil {
		return err
	}
	if err := saveConfig(config{Vault: dir}); err != nil {
		return err
	}
	runtime.EventsEmit(b.ctx, "menu", "vault-changed")
	return nil
}

// VaultPath tells the front end where the notes live.
func (b *Bridge) VaultPath() string {
	if v, err := b.vault(); err == nil {
		return v.Root()
	}
	return ""
}
