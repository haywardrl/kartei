package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// appMenu is the native menu. Items send a "menu" event with a command name;
// the front end maps commands to the same actions its buttons use.
func appMenu(b *Bridge) *menu.Menu {
	send := func(cmd string) func(*menu.CallbackData) {
		return func(*menu.CallbackData) { runtime.EventsEmit(b.ctx, "menu", cmd) }
	}
	m := menu.NewMenu()
	m.Append(menu.AppMenu())
	file := m.AddSubmenu("File")
	file.AddText("New card", keys.CmdOrCtrl("n"), send("new-card"))
	file.AddText("Open vault…", keys.CmdOrCtrl("o"), func(*menu.CallbackData) { _ = b.SwitchVault() })
	file.AddSeparator()
	file.AddText("Quit", keys.CmdOrCtrl("q"), func(*menu.CallbackData) { runtime.Quit(b.ctx) })
	m.Append(menu.EditMenu()) // copy, paste, select all: needed for typing on macOS
	view := m.AddSubmenu("View")
	view.AddText("Contents", keys.CmdOrCtrl("1"), send("contents"))
	view.AddText("The desk", keys.CmdOrCtrl("2"), send("desk"))
	view.AddText("Corkboard", keys.CmdOrCtrl("3"), send("board"))
	view.AddText("Register", keys.CmdOrCtrl("4"), send("register"))
	view.AddSeparator()
	view.AddText("Search", keys.CmdOrCtrl("f"), send("search"))
	view.AddText("Lamp", keys.CmdOrCtrl("l"), send("lamp"))
	view.AddText("Sound", nil, send("sound"))
	help := m.AddSubmenu("Help")
	help.AddText("How the study works", keys.Key("?"), send("help"))
	return m
}
