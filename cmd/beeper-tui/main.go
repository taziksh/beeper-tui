package main

import (
	"fmt"
	"os"

	"github.com/taziksh/beeper-tui/internal/config"
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
}
