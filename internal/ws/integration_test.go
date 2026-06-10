//go:build integration

package ws_test

import (
	"os"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/ws"
)

// Run with: go test -tags=integration ./internal/ws/...
// Requires Beeper Desktop running with the Desktop API enabled and
// BEEPER_ACCESS_TOKEN set in the environment.
//
// Logs event types and counts only, never message or chat content.
func TestIntegration_ConnectAndSubscribe(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Token == "" {
		t.Skip("BEEPER_ACCESS_TOKEN not set; skipping integration test")
	}

	c := ws.New(cfg)
	defer c.Close()

	deadline := time.After(15 * time.Second)
	for {
		select {
		case e, ok := <-c.Events():
			if !ok {
				t.Fatal("events channel closed before connecting")
			}
			switch e.Type {
			case ws.EventConnected:
				t.Log("connected and subscribed to live events")
				logEventCounts(t, c)
				return
			case ws.EventDisconnected:
				t.Logf("disconnected (will retry in %v): %v", e.RetryIn, e.Err)
			}
		case <-deadline:
			t.Fatal("never reached Connected against real Beeper Desktop")
		}
	}
}

// logEventCounts drains live events and logs per-type counts, confirming
// envelope decoding against the real wire format when any traffic arrives.
// BEEPER_TUI_WS_DRAIN overrides the default 5s window, leaving time to send
// a test message from another device.
func logEventCounts(t *testing.T, c *ws.Client) {
	window := 5 * time.Second
	if d, err := time.ParseDuration(os.Getenv("BEEPER_TUI_WS_DRAIN")); err == nil {
		window = d
	}
	counts := map[ws.EventType]int{}
	timeout := time.After(window)
	for {
		select {
		case e, ok := <-c.Events():
			if !ok {
				return
			}
			counts[e.Type]++
			if len(e.Entries) > 0 {
				counts["entries"] += len(e.Entries)
			}
		case <-timeout:
			t.Logf("events observed over %v: %v", window, counts)
			return
		}
	}
}
