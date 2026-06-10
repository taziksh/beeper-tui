//go:build integration

package ws_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/ws"
)

// TestIntegration_BridgeEventMatrix probes event emission per bridge without
// any manual messaging: it mark-reads one already-read chat per network and
// records which networks produce a WebSocket event for it. Networks that stay
// silent here are the ones the polling backstop exists for. Logs network
// names and booleans only.
func TestIntegration_BridgeEventMatrix(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Token == "" {
		t.Skip("BEEPER_ACCESS_TOKEN not set; skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rest := api.New(cfg)
	chats, err := rest.ListChats(ctx)
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	candidates := map[string]api.Chat{}
	for _, ch := range chats {
		if ch.Archived || ch.Unread > 0 || ch.MarkedUnread || ch.Network == "" {
			continue
		}
		if _, ok := candidates[ch.Network]; !ok {
			candidates[ch.Network] = ch
		}
	}
	if len(candidates) == 0 {
		t.Skip("no fully-read unarchived chats to probe with")
	}

	c := ws.New(cfg)
	defer c.Close()
	waitForConnected(t, c)

	networks := make([]string, 0, len(candidates))
	for net := range candidates {
		networks = append(networks, net)
	}
	sort.Strings(networks)

	results := map[string]bool{}
	for _, net := range networks {
		chat := candidates[net]
		if err := rest.MarkRead(ctx, chat.ID); err != nil {
			t.Logf("network %q: mark-read error, skipping: %v", net, err)
			continue
		}
		results[net] = waitForEventForChat(c, chat.ID, 8*time.Second)
		t.Logf("network %q: emitsEvents=%t", net, results[net])
	}

	silent := make([]string, 0)
	for net, ok := range results {
		if !ok {
			silent = append(silent, net)
		}
	}
	sort.Strings(silent)
	t.Logf("bridge event matrix: %v", results)
	t.Logf("silent bridges (covered only by the polling backstop): %v", silent)
}

func waitForConnected(t *testing.T, c *ws.Client) {
	t.Helper()
	deadline := time.After(15 * time.Second)
	for {
		select {
		case e, ok := <-c.Events():
			if !ok {
				t.Fatal("events channel closed before connecting")
			}
			if e.Type == ws.EventConnected {
				return
			}
		case <-deadline:
			t.Fatal("never connected")
		}
	}
}

// waitForEventForChat reports whether any domain event for chatID arrives
// within wait. Events for other chats are drained and ignored.
func waitForEventForChat(c *ws.Client, chatID string, wait time.Duration) bool {
	deadline := time.After(wait)
	for {
		select {
		case e, ok := <-c.Events():
			if !ok {
				return false
			}
			if e.ChatID == chatID {
				return true
			}
			for _, id := range e.IDs {
				if id == chatID {
					return true
				}
			}
		case <-deadline:
			return false
		}
	}
}
