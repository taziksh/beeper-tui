package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

// singlePageChatsJSON mirrors the real /v1/chats response shape (captured from
// the live API on 2026-05-20). Preview is a full shared.Message; only its
// "text" field matters to us.
const singlePageChatsJSON = `{
  "items": [
    {
      "id": "chat-1", "accountID": "acc-wa", "network": "WhatsApp",
      "title": "Hiking Group", "type": "group",
      "unreadCount": 82, "unreadMentionsCount": 0,
      "lastActivity": "2026-05-19T12:00:00Z",
      "preview": {"id":"m0","accountID":"acc-wa","chatID":"chat-1","senderID":"u1","sortKey":"1","timestamp":"2026-05-19T12:00:00Z","text":"see you there"}
    },
    {
      "id": "chat-2", "accountID": "acc-sig", "network": "Signal",
      "title": "Bob", "type": "single",
      "unreadCount": 0, "unreadMentionsCount": 0,
      "lastActivity": "2026-05-18T09:30:00Z",
      "preview": {"id":"m1","accountID":"acc-sig","chatID":"chat-2","senderID":"u2","sortKey":"2","timestamp":"2026-05-18T09:30:00Z","text":"ok!"}
    }
  ],
  "hasMore": false,
  "oldestCursor": "c-old", "newestCursor": "c-new"
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return api.New(config.Config{Token: "test", BaseURL: srv.URL})
}

func TestListChats_MapsSinglePage(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(singlePageChatsJSON))
	})

	chats, err := client.ListChats(context.Background())
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	if len(chats) != 2 {
		t.Fatalf("got %d chats, want 2", len(chats))
	}
	if chats[0].Title != "Hiking Group" {
		t.Errorf("chats[0].Title = %q, want %q", chats[0].Title, "Hiking Group")
	}
	if chats[0].Network != "WhatsApp" {
		t.Errorf("chats[0].Network = %q, want WhatsApp", chats[0].Network)
	}
	if chats[0].Unread != 82 {
		t.Errorf("chats[0].Unread = %d, want 82", chats[0].Unread)
	}
	if chats[0].Preview != "see you there" {
		t.Errorf("chats[0].Preview = %q, want 'see you there'", chats[0].Preview)
	}
	if chats[0].LastActive.IsZero() {
		t.Error("chats[0].LastActive is zero, want parsed timestamp")
	}
}
