package api_test

import (
	"context"
	"net/http"
	"testing"
)

const messagesJSON = `{
  "items": [
    {"id":"m1","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"1","text":"hey","timestamp":"2026-05-19T10:00:00Z","isSender":false,"senderName":"Bob"},
    {"id":"m2","accountID":"acc","chatID":"chat-1","senderID":"me","sortKey":"2","text":"yo","timestamp":"2026-05-19T10:01:00Z","isSender":true,"senderName":"Me"}
  ],
  "hasMore": false, "oldestCursor": "o", "newestCursor": "n"
}`

func TestListMessages_MapsMessages(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(messagesJSON))
	})

	msgs, err := client.ListMessages(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Text != "hey" {
		t.Errorf("msgs[0].Text = %q, want hey", msgs[0].Text)
	}
	if msgs[0].SenderName != "Bob" {
		t.Errorf("msgs[0].SenderName = %q, want Bob", msgs[0].SenderName)
	}
	if !msgs[1].IsFromMe {
		t.Error("msgs[1].IsFromMe = false, want true")
	}
	if msgs[0].Timestamp.IsZero() {
		t.Error("msgs[0].Timestamp is zero, want parsed")
	}
}

func TestListMessages_SortsOldestFirst(t *testing.T) {
	// Items deliberately newest-first in the payload; output must be oldest-first.
	const reversedJSON = `{
	  "items": [
	    {"id":"m2","accountID":"acc","chatID":"chat-1","senderID":"me","sortKey":"2","text":"newer","timestamp":"2026-05-19T10:01:00Z","isSender":true,"senderName":"Me"},
	    {"id":"m1","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"1","text":"older","timestamp":"2026-05-19T10:00:00Z","isSender":false,"senderName":"Bob"}
	  ],
	  "hasMore": false, "oldestCursor": "o", "newestCursor": "n"
	}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reversedJSON))
	})

	msgs, err := client.ListMessages(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Text != "older" || msgs[1].Text != "newer" {
		t.Errorf("order = [%q, %q], want [older, newer]", msgs[0].Text, msgs[1].Text)
	}
}
