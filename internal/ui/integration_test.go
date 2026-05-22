//go:build integration

package ui

import (
	"context"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

// Run with: go test -tags=integration ./internal/ui/...
// Requires Beeper Desktop running + BEEPER_ACCESS_TOKEN. Asserts COUNTS only —
// never prints chat or message content (data rule).
func TestIntegration_LoadChatsThenMessages(t *testing.T) {
	cfg, err := config.Load()
	if err != nil || cfg.Token == "" {
		t.Skip("no token / config; skipping integration test")
	}
	client := api.New(cfg)
	chats, err := client.ListChats(context.Background())
	if err != nil {
		t.Fatalf("ListChats: %v", err)
	}
	if len(chats) == 0 {
		t.Fatal("expected at least one chat")
	}
	msgs, err := client.ListMessages(context.Background(), chats[0].ID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	t.Logf("loaded %d chats; first chat has %d messages", len(chats), len(msgs))
}
