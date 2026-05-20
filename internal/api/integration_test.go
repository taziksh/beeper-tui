//go:build integration

package api_test

import (
	"context"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

// Run with: go test -tags=integration ./internal/api/...
// Requires Beeper Desktop running with the Desktop API enabled and
// BEEPER_ACCESS_TOKEN set in the environment.
func TestIntegration_ListChats(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Token == "" {
		t.Skip("BEEPER_ACCESS_TOKEN not set; skipping integration test")
	}

	client := api.New(cfg)
	chats, err := client.ListChats(context.Background())
	if err != nil {
		t.Fatalf("ListChats() against real API error = %v", err)
	}
	if len(chats) == 0 {
		t.Fatal("ListChats() returned 0 chats from real API; expected at least one")
	}
	t.Logf("fetched %d chats from real Beeper Desktop", len(chats))
}
