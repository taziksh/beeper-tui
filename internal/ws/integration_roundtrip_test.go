//go:build integration

package ws

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

// TestIntegration_LiveEventRoundTrip is fully self-contained: it listens on
// two parallel connections — one raw socket and one Client — then triggers a
// real event itself through the REST API and compares what each connection
// observes. This discriminates between "the server never sent events to us"
// and "the client received frames but dropped them".
//
// Triggers used: mark-read on an already-read chat first; if that produces no
// events, archive then unarchive the same chat, restoring its state. Logs
// only frame types, sizes, and decode results, never content or chat IDs.
func TestIntegration_LiveEventRoundTrip(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Token == "" {
		t.Skip("BEEPER_ACCESS_TOKEN not set; skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var rawDomain, clientDomain atomic.Int64

	startRawListener(ctx, t, cfg, &rawDomain)

	c := New(cfg)
	defer c.Close()
	waitConnected(ctx, t, c)
	go func() {
		for e := range c.Events() {
			switch e.Type {
			case EventConnecting, EventConnected, EventDisconnected:
				t.Logf("client: state %q err=%v", e.Type, e.Err)
			default:
				t.Logf("client: event type=%q seq=%d entries=%d ts=%s", e.Type, e.Seq, len(e.Entries), e.TS.Format(time.RFC3339))
				clientDomain.Add(1)
			}
		}
	}()

	rest := api.New(cfg)
	chatID := pickReadChat(ctx, t, rest)

	t.Log("trigger 1: mark-read on an already-read chat")
	if err := rest.MarkRead(ctx, chatID); err != nil {
		t.Logf("MarkRead error (continuing): %v", err)
	}
	if observed(ctx, 10*time.Second, &rawDomain, &clientDomain) {
		report(t, &rawDomain, &clientDomain)
		return
	}

	t.Log("trigger 2: archive then unarchive the same chat")
	if err := rest.ArchiveChat(ctx, chatID, true); err != nil {
		t.Fatalf("ArchiveChat(true) error = %v", err)
	}
	time.Sleep(2 * time.Second)
	if err := rest.ArchiveChat(ctx, chatID, false); err != nil {
		t.Errorf("ArchiveChat(false) restore error = %v — chat left archived", err)
	}
	observed(ctx, 15*time.Second, &rawDomain, &clientDomain)
	report(t, &rawDomain, &clientDomain)
}

// startRawListener dials the socket directly, subscribes, and counts domain
// frames, logging each frame's type and our decoder's verdict on it.
func startRawListener(ctx context.Context, t *testing.T, cfg config.Config, domain *atomic.Int64) {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, cfg.BaseURL+"/v1/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + cfg.Token}},
	})
	if err != nil {
		t.Fatalf("raw dial error = %v", err)
	}
	conn.SetReadLimit(8 << 20)
	go func() {
		defer func() { _ = conn.CloseNow() }()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			frame, decErr := decodeFrame(data)
			switch f := frame.(type) {
			case ready:
				t.Log("raw: ready; subscribing")
				cmd := subscriptionsSet{Type: "subscriptions.set", RequestID: "raw1", ChatIDs: []string{"*"}}
				if err := wsjson.Write(ctx, conn, cmd); err != nil {
					t.Logf("raw: subscribe error: %v", err)
					return
				}
			case subscriptionsUpdated:
				t.Log("raw: subscription acked")
			case serverError:
				t.Logf("raw: server error code=%q message=%q", f.Code, f.Message)
			case Event:
				t.Logf("raw: event type=%q seq=%d entries=%d size=%dB decode=ok", f.Type, f.Seq, len(f.Entries), len(data))
				domain.Add(1)
			default:
				t.Logf("raw: frame %dB decode FAILED: %v", len(data), decErr)
			}
		}
	}()
}

func waitConnected(ctx context.Context, t *testing.T, c *Client) {
	t.Helper()
	deadline := time.After(15 * time.Second)
	for {
		select {
		case e, ok := <-c.Events():
			if !ok {
				t.Fatal("events channel closed before connecting")
			}
			if e.Type == EventConnected {
				t.Log("client: connected")
				return
			}
		case <-deadline:
			t.Fatal("client never reached Connected")
		case <-ctx.Done():
			t.Fatal("context expired before Connected")
		}
	}
}

// pickReadChat returns the most recently active unarchived chat with no
// unread state, so mark-read is a no-op and an archive round-trip is least
// disruptive.
func pickReadChat(ctx context.Context, t *testing.T, rest *api.Client) string {
	t.Helper()
	chats, err := rest.ListChats(ctx)
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	for _, ch := range chats {
		if !ch.Archived && ch.Unread == 0 && !ch.MarkedUnread {
			return ch.ID
		}
	}
	t.Skip("no fully-read unarchived chat available to use as trigger target")
	return ""
}

// observed polls both counters until either sees a domain event or the wait
// elapses.
func observed(ctx context.Context, wait time.Duration, raw, client *atomic.Int64) bool {
	deadline := time.After(wait)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if raw.Load() > 0 || client.Load() > 0 {
				// Give the slower connection a moment to catch up.
				time.Sleep(2 * time.Second)
				return true
			}
		case <-deadline:
			return false
		case <-ctx.Done():
			return false
		}
	}
}

func report(t *testing.T, raw, client *atomic.Int64) {
	t.Helper()
	r, c := raw.Load(), client.Load()
	t.Logf("RESULT: raw connection saw %d domain frames; client delivered %d events", r, c)
	switch {
	case r == 0 && c == 0:
		t.Error("neither connection observed events — triggers fired none, or subscriptions are broken")
	case r > 0 && c == 0:
		t.Error("server sent frames but the client dropped them — client-side bug confirmed")
	case c > 0:
		t.Log("client path verified end to end")
	}
}
