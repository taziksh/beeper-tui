package ws_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/ws"
)

// setCommand mirrors the client's subscriptions.set wire format for asserting
// on what the client sends.
type setCommand struct {
	Type      string   `json:"type"`
	RequestID string   `json:"requestID"`
	ChatIDs   []string `json:"chatIDs"`
}

// newServer starts an httptest server that upgrades each request to a
// WebSocket and hands the connection to handler. handler runs on its own
// goroutine per connection.
func newServer(t *testing.T, handler func(ctx context.Context, conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		handler(r.Context(), conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newClient(t *testing.T, srv *httptest.Server, opts ...ws.Option) *ws.Client {
	t.Helper()
	opts = append([]ws.Option{ws.WithBackoff([]time.Duration{10 * time.Millisecond})}, opts...)
	c := ws.New(config.Config{Token: "test-token", BaseURL: srv.URL}, opts...)
	t.Cleanup(c.Close)
	return c
}

// waitFor reads events until one of the wanted type arrives, failing the test
// on timeout or channel close.
func waitFor(t *testing.T, c *ws.Client, want ws.EventType) ws.Event {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-c.Events():
			if !ok {
				t.Fatalf("events channel closed while waiting for %q", want)
			}
			if e.Type == want {
				return e
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q event", want)
		}
	}
}

func sendReady(ctx context.Context, conn *websocket.Conn) {
	_ = wsjson.Write(ctx, conn, map[string]any{"type": "ready", "version": 1, "chatIDs": []string{}})
}

func TestClient_SubscribesAfterReady(t *testing.T) {
	gotSet := make(chan setCommand, 1)
	srv := newServer(t, func(ctx context.Context, conn *websocket.Conn) {
		sendReady(ctx, conn)
		var cmd setCommand
		if err := wsjson.Read(ctx, conn, &cmd); err != nil {
			return
		}
		gotSet <- cmd
		<-ctx.Done()
	})
	c := newClient(t, srv)

	waitFor(t, c, ws.EventConnected)
	select {
	case cmd := <-gotSet:
		if cmd.Type != "subscriptions.set" {
			t.Errorf("Type = %q, want subscriptions.set", cmd.Type)
		}
		if cmd.RequestID == "" {
			t.Error("RequestID is empty")
		}
		if len(cmd.ChatIDs) != 1 || cmd.ChatIDs[0] != "*" {
			t.Errorf("ChatIDs = %v, want [*]", cmd.ChatIDs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server never received subscriptions.set")
	}
}

func TestClient_DeliversDomainEvents(t *testing.T) {
	srv := newServer(t, func(ctx context.Context, conn *websocket.Conn) {
		sendReady(ctx, conn)
		var cmd setCommand
		if err := wsjson.Read(ctx, conn, &cmd); err != nil {
			return
		}
		_ = conn.Write(ctx, websocket.MessageText, []byte(
			`{"type":"message.upserted","seq":7,"ts":1739320000000,"chatID":"chat_a","ids":["m1"],"entries":[{"id":"m1","text":"hello"}]}`))
		<-ctx.Done()
	})
	c := newClient(t, srv)

	e := waitFor(t, c, ws.EventMessageUpserted)
	if e.Seq != 7 {
		t.Errorf("Seq = %d, want 7", e.Seq)
	}
	if e.ChatID != "chat_a" {
		t.Errorf("ChatID = %q, want chat_a", e.ChatID)
	}
	if len(e.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(e.Entries))
	}
	var entry struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(e.Entries[0], &entry); err != nil || entry.ID != "m1" {
		t.Errorf("Entries[0] = %s, want object with id m1 (err %v)", e.Entries[0], err)
	}
}

func TestClient_SendsBearerToken(t *testing.T) {
	gotAuth := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth <- r.Header.Get("Authorization")
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		sendReady(r.Context(), conn)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	c := newClient(t, srv)

	waitFor(t, c, ws.EventConnecting)
	select {
	case auth := <-gotAuth:
		if auth != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server never received a request")
	}
}

func TestClient_ReconnectsAfterServerClose(t *testing.T) {
	var dials atomic.Int32
	srv := newServer(t, func(ctx context.Context, conn *websocket.Conn) {
		n := dials.Add(1)
		sendReady(ctx, conn)
		var cmd setCommand
		if err := wsjson.Read(ctx, conn, &cmd); err != nil {
			return
		}
		if n == 1 {
			conn.Close(websocket.StatusGoingAway, "bye")
			return
		}
		<-ctx.Done()
	})
	c := newClient(t, srv)

	waitFor(t, c, ws.EventConnected)
	e := waitFor(t, c, ws.EventDisconnected)
	if e.Err == nil {
		t.Error("Disconnected event has nil Err")
	}
	if e.RetryIn != 10*time.Millisecond {
		t.Errorf("RetryIn = %v, want 10ms", e.RetryIn)
	}
	waitFor(t, c, ws.EventConnected)
	if got := dials.Load(); got < 2 {
		t.Errorf("dials = %d, want >= 2", got)
	}
}

func TestClient_RetrySkipsBackoff(t *testing.T) {
	var dials atomic.Int32
	srv := newServer(t, func(ctx context.Context, conn *websocket.Conn) {
		n := dials.Add(1)
		sendReady(ctx, conn)
		var cmd setCommand
		if err := wsjson.Read(ctx, conn, &cmd); err != nil {
			return
		}
		if n == 1 {
			conn.Close(websocket.StatusGoingAway, "bye")
			return
		}
		<-ctx.Done()
	})
	c := newClient(t, srv, ws.WithBackoff([]time.Duration{time.Hour}))

	waitFor(t, c, ws.EventConnected)
	waitFor(t, c, ws.EventDisconnected)
	c.Retry()
	waitFor(t, c, ws.EventConnected)
}

func TestClient_SetSubscriptionsSendsNewSet(t *testing.T) {
	sets := make(chan setCommand, 2)
	srv := newServer(t, func(ctx context.Context, conn *websocket.Conn) {
		sendReady(ctx, conn)
		for {
			var cmd setCommand
			if err := wsjson.Read(ctx, conn, &cmd); err != nil {
				return
			}
			sets <- cmd
		}
	})
	c := newClient(t, srv)

	waitFor(t, c, ws.EventConnected)
	<-sets
	c.SetSubscriptions([]string{"chat_a", "chat_b"})
	select {
	case cmd := <-sets:
		if len(cmd.ChatIDs) != 2 || cmd.ChatIDs[0] != "chat_a" || cmd.ChatIDs[1] != "chat_b" {
			t.Errorf("ChatIDs = %v, want [chat_a chat_b]", cmd.ChatIDs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server never received the updated subscriptions.set")
	}
}

func TestClient_CloseClosesEventsChannel(t *testing.T) {
	srv := newServer(t, func(ctx context.Context, conn *websocket.Conn) {
		sendReady(ctx, conn)
		<-ctx.Done()
	})
	c := ws.New(config.Config{Token: "test-token", BaseURL: srv.URL},
		ws.WithBackoff([]time.Duration{10 * time.Millisecond}))

	c.Close()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-c.Events():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("events channel not closed after Close()")
		}
	}
}

func TestClient_SkipsUnknownFrames(t *testing.T) {
	srv := newServer(t, func(ctx context.Context, conn *websocket.Conn) {
		sendReady(ctx, conn)
		var cmd setCommand
		if err := wsjson.Read(ctx, conn, &cmd); err != nil {
			return
		}
		_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"presence.updated"}`))
		_ = conn.Write(ctx, websocket.MessageText, []byte(
			`{"type":"chat.deleted","seq":9,"ts":1,"chatID":"chat_x","ids":["chat_x"]}`))
		<-ctx.Done()
	})
	c := newClient(t, srv)

	e := waitFor(t, c, ws.EventChatDeleted)
	if e.ChatID != "chat_x" {
		t.Errorf("ChatID = %q, want chat_x", e.ChatID)
	}
}
