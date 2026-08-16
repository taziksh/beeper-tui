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
	"github.com/taziksh/beeper-tui/internal/identity"
	"github.com/taziksh/beeper-tui/internal/launch"
	"github.com/taziksh/beeper-tui/internal/llm"
	"github.com/taziksh/beeper-tui/internal/person"
	"github.com/taziksh/beeper-tui/internal/redact"
	"github.com/taziksh/beeper-tui/internal/state"
	"github.com/taziksh/beeper-tui/internal/tinfoil"
	"github.com/taziksh/beeper-tui/internal/ui"
	"github.com/taziksh/beeper-tui/internal/ws"
)

func main() {
	unansweredDebug := flag.Bool("unanswered-debug", false, "print per-chat unanswered-filter signals (no names or text) and exit")
	verifyTinfoil := flag.Bool("verify-tinfoil", false, "attest the Tinfoil enclave, print the verification document, and exit")
	flag.Parse()

	if *verifyTinfoil {
		if err := tinfoil.PrintVerification(os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

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

	// Cross-network merge decisions live in a hand-edited sidecar; new
	// ambiguous pairs found at startup are written back as pending.
	mergePath := filepath.Join(cfg.ConfigDir, "identity-merges.yaml")
	merges, err := identity.LoadMergePolicy(mergePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// The vault is built before anything can reach the model, so every
	// outbound prompt carries tokens instead of known identities.
	vault := redact.SessionVault(context.Background(), client, merges)
	llmOpts := []llm.Option{llm.WithRedactor(vault)}
	if err := identity.SaveMergePolicy(mergePath, merges); err != nil {
		fmt.Fprintf(os.Stderr, "identity merges: %v\n", err)
	}
	if cfg.LLMProvider == config.ProviderTinfoil {
		fmt.Fprintln(os.Stderr, "verifying tinfoil enclave…")
		hc, err := tinfoil.Dial()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		llmOpts = append(llmOpts, llm.WithHTTPClient(hc), llm.WithAPIKey(cfg.TinfoilAPIKey))
	}
	assistant := llm.New(cfg.LLMBaseURL, cfg.LLMModel, llmOpts...)

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
	go person.Sweep(sweepCtx, client, assistant, people, 10, merges)

	final, err := tea.NewProgram(ui.New(client, events).WithCache(cached, cachePath).WithLLM(assistant).WithPeople(people).WithMerges(merges).WithVault(vault)).Run()
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
