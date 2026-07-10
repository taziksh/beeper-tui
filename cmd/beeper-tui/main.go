package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/identity"
	"github.com/taziksh/beeper-tui/internal/launch"
	"github.com/taziksh/beeper-tui/internal/state"
	"github.com/taziksh/beeper-tui/internal/ui"
	"github.com/taziksh/beeper-tui/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "No BEEPER_ACCESS_TOKEN set. Enable the Desktop API in Beeper (Settings -> Developers -> Approved connections) and export a token.")
		os.Exit(1)
	}

	if err := launch.EnsureRunning(context.Background(), cfg.BaseURL); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	client := api.New(cfg)
	events := ws.New(cfg)
	defer events.Close()

	// A cache that is missing, corrupt, or from an old schema just means a
	// cold start. An unwritable cache dir disables caching for the session.
	cachePath := filepath.Join(cfg.CacheDir, "cache.json")
	if err := os.MkdirAll(cfg.CacheDir, 0o700); err != nil {
		cachePath = ""
	}
	cached, _ := state.Load(cachePath)

	// Person cards are durable user data under the config dir (not the chat cache).
	identsPath := filepath.Join(cfg.ConfigDir, identity.FileName)
	idents, err := identity.Load(identsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "identities: %v (continuing without person cards)\n", err)
		idents = nil
	}

	final, err := tea.NewProgram(ui.New(client, events).WithCache(cached, cachePath).WithIdentities(idents)).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
	if m, ok := final.(ui.Model); ok && cachePath != "" {
		if snap := m.Snapshot(); len(snap.Chats) > 0 {
			_ = state.Save(cachePath, snap)
		}
	}
}
