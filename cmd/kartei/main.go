// Command kartei opens a vault and serves the room on localhost.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/haywardrl/kartei/internal/server"
	"github.com/haywardrl/kartei/internal/slipbox"
	"github.com/haywardrl/kartei/ui"
)

func main() {
	home, _ := os.UserHomeDir()
	vaultFlag := flag.String("vault", filepath.Join(home, "Kartei"), "vault folder (created if absent)")
	demo := flag.Bool("demo", false, "seed a demo vault in a temp folder, print its drawers")
	serve := flag.Bool("serve", false, "with --demo: open the room on the demo vault instead of exiting")
	port := flag.Int("port", 0, "port to listen on (0 picks a free one)")
	noOpen := flag.Bool("no-open", false, "do not open the browser")
	tokenFlag := flag.String("token", "", "API token to require (default: a random one, printed at start); a front end that is not the bundled room passes it in X-Kartei-Token")
	flag.Parse()

	root := *vaultFlag
	if *demo {
		dir, err := os.MkdirTemp("", "kartei-demo-*")
		if err != nil {
			fatal(err)
		}
		if err := seedDemo(dir); err != nil {
			fatal(err)
		}
		root = dir
	}

	v, err := slipbox.Open(root)
	if err != nil {
		fatal(err)
	}
	defer v.Close()

	if *demo {
		printDrawers(v)
		if !*serve {
			return
		}
	}
	if err := v.Watch(); err != nil {
		fmt.Fprintln(os.Stderr, "file watching unavailable:", err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fatal(err)
	}
	url := "http://" + ln.Addr().String()
	fmt.Printf("kartei: vault %s\nkartei: room at %s\n", v.Root(), url)

	token := *tokenFlag
	if token == "" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			fatal(err)
		}
		token = hex.EncodeToString(raw)
	}
	fmt.Printf("kartei: token %s\n", token)
	srv := &http.Server{Handler: server.New(v, ui.FS, token), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fatal(err)
		}
	}()
	if !*noOpen {
		openBrowser(url)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("\nkartei: saving and closing")
	srv.Close()
}

func printDrawers(v *slipbox.Vault) {
	fmt.Printf("vault: %s  (%d notes, stage: %s)\n", v.Root(), len(v.Notes()), v.Stage())
	for _, d := range v.Drawers() {
		fmt.Printf("\nDrawer %d  [%s]  %d cards\n", d.Index+1, d.Label, len(d.IDs))
		for _, id := range d.IDs {
			n, _ := v.Note(id)
			fmt.Printf("  %-8s %s\n", v.Address(id), n.Title)
		}
	}
	if ws := v.Warnings(); len(ws) > 0 {
		fmt.Printf("\n%d warnings\n", len(ws))
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "kartei:", err)
	os.Exit(1)
}
