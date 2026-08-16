// Command chat-eval runs the chat assistant's common queries end to end —
// real local model, stub or configured API — and prints each answer with its
// tool trace for a human to judge. Person cards go to a throwaway directory
// so evals never touch real cards.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/llm"
	"github.com/taziksh/beeper-tui/internal/person"
	"github.com/taziksh/beeper-tui/internal/redact"
	"github.com/taziksh/beeper-tui/internal/ui"
)

// queries are the common asks the assistant must handle well. Add a line
// here whenever a live failure shows a new pattern worth guarding.
var queries = []string{
	"who have i forgotten to reply to?",
	"who texted me yesterday?",
	"tell me about dana",
	"what does dana like?",
	"where does dana live?",
	"did anyone mention money recently?",
	"who do i know in toronto?",
}

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	client := api.New(cfg)
	lc := llm.New(cfg.LLMBaseURL, cfg.LLMModel, llm.WithRedactor(redact.SessionVault(ctx, client)))
	if lc.Model() == "" {
		if _, err := lc.DetectModel(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "llm: %v (is the model server at %s running?)\n", err, cfg.LLMBaseURL)
			os.Exit(1)
		}
	}
	dir, err := os.MkdirTemp("", "beeper-eval-people-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "tempdir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	people, err := person.NewStore(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "person store: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("model:", lc.Model())
	fmt.Println("sweeping person cards…")
	person.Sweep(ctx, client, lc, people, 5)

	for _, q := range queries {
		fmt.Println("Q:", q)
		answer, trace, err := ui.RunChatQuery(ctx, lc, client, people, q)
		for _, line := range trace {
			fmt.Println("   " + line)
		}
		if err != nil {
			fmt.Println("   error:", err)
		}
		fmt.Println("A:", answer)
		fmt.Println()
	}
}
