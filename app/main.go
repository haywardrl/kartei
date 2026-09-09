// Command app is Kartei as a native desktop application: the same room,
// the same engine, in a Wails window with no server and no token.
package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/haywardrl/kartei/ui"
)

func main() {
	b := &Bridge{}
	err := wails.Run(&options.App{
		Title:            "Kartei",
		Width:            1280,
		Height:           800,
		MinWidth:         960,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: ui.FS},
		BackgroundColour: &options.RGBA{R: 24, G: 20, B: 37, A: 255},
		Menu:             appMenu(b),
		Bind:             []any{b},
		OnStartup: func(ctx context.Context) {
			b.ctx = ctx
			if err := b.openVault(); err != nil {
				log.Println("open vault:", err)
			}
		},
		OnShutdown: func(ctx context.Context) { b.closeVault() },
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
			About:    &mac.AboutInfo{Title: "Kartei", Message: "A zettelkasten you keep in a cosy study."},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
