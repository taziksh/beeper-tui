package ui

import (
	"context"
	"strings"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/llm"
	"github.com/taziksh/beeper-tui/internal/person"
)

// RunChatQuery answers one question through the full tool loop without the
// TUI. It exists for eval harnesses: the answer and tool trace come back as
// plain strings for a human to judge.
func RunChatQuery(ctx context.Context, lc *llm.Client, client *api.Client, people *person.Store, question string) (answer string, trace []string, err error) {
	msgs := []llm.Message{
		{Role: "system", Content: chatSystemPrompt()},
		{Role: "user", Content: question},
	}
	events := make(chan chatEvent, 64)
	go runChatSession(ctx, lc, toolEnv{client: client, llm: lc, people: people}, msgs, events)
	var b strings.Builder
	for ev := range events {
		switch ev.kind {
		case chatEvToken:
			if b.Len() == 0 {
				ev.text = strings.TrimLeft(ev.text, " \n\t")
			}
			b.WriteString(ev.text)
		case chatEvToolEnd:
			trace = append(trace, strings.TrimSpace(chatStepLine(ev.step)))
		case chatEvErr:
			return b.String(), trace, ev.err
		}
	}
	return b.String(), trace, nil
}
