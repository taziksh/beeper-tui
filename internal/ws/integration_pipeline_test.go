//go:build integration

package ws_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/ws"
)

// TestIntegration_MessagePipeline validates the full live-message path the
// TUI depends on: a real message.upserted event must decode through
// api.MessageFromJSON and reference a chat ID that REST ListChats also
// reports, or the UI reducer cannot apply it. Requires a message to arrive
// while it listens (send yourself one). Logs field-presence booleans and
// decode errors only, never content.
func TestIntegration_MessagePipeline(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Token == "" {
		t.Skip("BEEPER_ACCESS_TOKEN not set; skipping integration test")
	}
	window := 60 * time.Second
	if d, err := time.ParseDuration(os.Getenv("BEEPER_TUI_WS_DRAIN")); err == nil {
		window = d
	}

	ctx, cancel := context.WithTimeout(context.Background(), window+30*time.Second)
	defer cancel()

	rest := api.New(cfg)
	chats, err := rest.ListChats(ctx)
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	network := make(map[string]string, len(chats))
	for _, c := range chats {
		network[c.ID] = c.Network
	}
	t.Logf("REST reports %d chats", len(chats))
	seen := map[string]int{}

	c := ws.New(cfg)
	defer c.Close()

	deadline := time.After(window)
	t.Log("listening — send yourself a message now")
	for {
		select {
		case e, ok := <-c.Events():
			if !ok {
				t.Fatal("events channel closed")
			}
			switch e.Type {
			case ws.EventConnected:
				t.Log("connected")
			case ws.EventDisconnected:
				t.Logf("disconnected: %v", e.Err)
			case ws.EventMessageUpserted:
				net, known := network[e.ChatID]
				t.Logf("message.upserted: seq=%d entries=%d network=%q envelopeChatIDInREST=%t",
					e.Seq, len(e.Entries), net, known)
				for i, raw := range e.Entries {
					msg, err := api.MessageFromJSON(raw)
					if err != nil {
						t.Errorf("entry %d: MessageFromJSON error = %v", i, err)
						continue
					}
					_, msgChatKnown := network[msg.ChatID]
					t.Logf("entry %d decoded: hasID=%t hasChatID=%t chatIDInREST=%t hasText=%t hasTimestamp=%t isFromMe=%t isUnread=%t",
						i, msg.ID != "", msg.ChatID != "", msgChatKnown,
						msg.Text != "", !msg.Timestamp.IsZero(), msg.IsFromMe, msg.IsUnread)
					if msg.ID == "" || msg.ChatID == "" {
						t.Error("entry missing ID or ChatID; the reducer cannot apply it")
					}
					if !msgChatKnown && !known {
						t.Error("event chat ID matches no REST chat ID; the reducer treats every message as an unknown chat")
					}
					seen[net]++
				}
			case ws.EventChatUpserted, ws.EventChatDeleted, ws.EventMessageDeleted:
				net := network[eventChatIDForLog(e)]
				t.Logf("other event: %s network=%q", e.Type, net)
			}
		case <-deadline:
			t.Logf("window over; message.upserted entries by network: %v", seen)
			if len(seen) == 0 {
				t.Error("no message.upserted arrived; if messages were sent during the window, event emission is broken upstream")
			}
			return
		}
	}
}

func eventChatIDForLog(e ws.Event) string {
	if e.ChatID != "" {
		return e.ChatID
	}
	if len(e.IDs) > 0 {
		return e.IDs[0]
	}
	return ""
}
