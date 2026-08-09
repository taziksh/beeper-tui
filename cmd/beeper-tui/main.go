package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/launch"
	"github.com/taziksh/beeper-tui/internal/llm"
	"github.com/taziksh/beeper-tui/internal/person"
	"github.com/taziksh/beeper-tui/internal/state"
	"github.com/taziksh/beeper-tui/internal/ui"
	"github.com/taziksh/beeper-tui/internal/ws"
)

func main() {
	unansweredDebug := flag.Bool("unanswered-debug", false, "print per-chat unanswered-filter signals (no names or text) and exit")
	flag.Parse()

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

	if *unansweredDebug {
		if err := ui.PrintUnansweredDebug(context.Background(), client, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	events := ws.New(cfg)
	defer events.Close()

	// A cache that is missing, corrupt, or from an old schema just means a
	// cold start. An unwritable cache dir disables caching for the session.
	cachePath := filepath.Join(cfg.CacheDir, "cache.json")
	if err := os.MkdirAll(cfg.CacheDir, 0o700); err != nil {
		cachePath = ""
	}
	cached, _ := state.Load(cachePath)

	assistant := llm.New(cfg.LLMBaseURL, cfg.LLMModel)

	// Person cards live with config, not cache: they are user data, edited by
	// hand as much as by extraction.
	people, err := person.NewStore(filepath.Join(cfg.ConfigDir, "people"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "person store: %v\n", err)
		os.Exit(1)
	}

	// Background sweep: keep card gaps filled from recent messages without
	// anyone asking. Best-effort; dies with the program.
	sweepCtx, stopSweep := context.WithCancel(context.Background())
	defer stopSweep()
	go person.Sweep(sweepCtx, client, assistant, people, 10)

	final, err := tea.NewProgram(ui.New(client, events).WithCache(cached, cachePath).WithLLM(assistant).WithPeople(people)).Run()
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
