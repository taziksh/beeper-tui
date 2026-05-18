package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/state"
)

const version = "0.0.0-phase-1"

func main() {
	fmt.Printf("beeper-tui %s\n", version)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	tokenStatus := "not set"
	if cfg.Token != "" {
		tokenStatus = "set"
	}

	fmt.Printf("  api base url: %s\n", cfg.BaseURL)
	fmt.Printf("  config dir:   %s\n", cfg.ConfigDir)
	fmt.Printf("  cache dir:    %s\n", cfg.CacheDir)
	fmt.Printf("  token:        %s\n", tokenStatus)

	cachePath := filepath.Join(cfg.CacheDir, "cache.json")
	cache, err := state.Load(cachePath)
	switch {
	case err == nil:
		fmt.Printf("  cache:        loaded %d chats\n", len(cache.Chats))
	case errors.Is(err, state.ErrCorruptCache):
		fmt.Printf("  cache:        corrupt, starting fresh\n")
	case errors.Is(err, state.ErrSchemaMismatch):
		fmt.Printf("  cache:        schema mismatch, starting fresh\n")
	default:
		fmt.Fprintf(os.Stderr, "cache: %v\n", err)
		os.Exit(1)
	}
}
