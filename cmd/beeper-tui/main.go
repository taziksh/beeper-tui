package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/state"
)

const version = "0.0.0-phase-2"

func main() {
	fmt.Printf("beeper-tui %s\n", version)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "No BEEPER_ACCESS_TOKEN set. Enable the Desktop API in Beeper (Settings -> Developers -> Approved connections) and export a token.")
		os.Exit(1)
	}

	cachePath := filepath.Join(cfg.CacheDir, "cache.json")
	if cache, err := state.Load(cachePath); err == nil {
		fmt.Printf("  cache: %d chats (warm)\n", len(cache.Chats))
	} else if !errors.Is(err, state.ErrCorruptCache) && !errors.Is(err, state.ErrSchemaMismatch) {
		fmt.Fprintf(os.Stderr, "cache: %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := api.New(cfg)
	chats, err := client.ListChats(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch chats: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n%d chats:\n", len(chats))
	for _, ch := range chats {
		marker := " "
		if ch.Unread > 0 {
			marker = "*"
		}
		fmt.Printf("  %s [%-10s] %3d  %s\n", marker, ch.Network, ch.Unread, ch.Title)
	}
}
