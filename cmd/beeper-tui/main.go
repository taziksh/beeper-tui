package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
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

	final, err := tea.NewProgram(ui.New(client, events).WithCache(cached, cachePath)).Run()
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
