package api_test

import (
	"context"
	"net/http"
	"strings"
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

func TestListMessages_EncodesHashInChatID(t *testing.T) {
	// iMessage chat IDs contain '#'. Unencoded, url.Parse treats it as a URL
	// fragment, so the server only receives "imsg" and returns 404 (bd-nz5).
	const chatID = "imsg##thread:abc123"
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"hasMore":false,"oldestCursor":"","newestCursor":""}`))
	})

	if _, err := client.ListMessages(context.Background(), chatID); err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	want := "/v1/chats/" + chatID + "/messages"
	if gotPath != want {
		t.Errorf("server received path %q, want %q (a '#' chat id must survive encoding)", gotPath, want)
	}
}

func TestListMessages_DecodesHTMLEntities(t *testing.T) {
	// Message text arrives HTML-escaped from the API; the conversation view
	// must render the decoded characters, not literal entities (bd-on8).
	const escapedJSON = `{
	  "items": [
	    {"id":"m1","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"1","text":"she said &quot;hi &amp; bye&quot; &lt;3 it&#39;s fine","timestamp":"2026-05-19T10:00:00Z","isSender":false,"senderName":"Bob"}
	  ],
	  "hasMore": false, "oldestCursor": "o", "newestCursor": "n"
	}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(escapedJSON))
	})

	msgs, err := client.ListMessages(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	want := `she said "hi & bye" <3 it's fine`
	if msgs[0].Text != want {
		t.Errorf("msgs[0].Text = %q, want %q", msgs[0].Text, want)
	}
}

func TestListMessages_MapsIsUnread(t *testing.T) {
	const json = `{
	  "items": [
	    {"id":"m1","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"1","text":"old","timestamp":"2026-05-19T10:00:00Z","isSender":false,"senderName":"Bob","isUnread":false},
	    {"id":"m2","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"2","text":"new","timestamp":"2026-05-19T10:01:00Z","isSender":false,"senderName":"Bob","isUnread":true}
	  ],
	  "hasMore": false, "oldestCursor": "o", "newestCursor": "n"
	}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(json))
	})

	msgs, err := client.ListMessages(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if msgs[0].IsUnread {
		t.Error("msgs[0].IsUnread = true, want false")
	}
	if !msgs[1].IsUnread {
		t.Error("msgs[1].IsUnread = false, want true")
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

func TestSearchMessages_UsesQueryAndMapsResults(t *testing.T) {
	var gotQuery string
	var gotLimit string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages/search" {
			t.Errorf("path = %q, want /v1/messages/search", r.URL.Path)
		}
		gotQuery = r.URL.Query().Get("query")
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "items": [
		    {"id":"m1","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"1","text":"Dinner at 7?","timestamp":"2026-05-19T10:00:00Z","isSender":false,"senderName":"Bob"},
		    {"id":"m2","accountID":"acc","chatID":"chat-2","senderID":"me","sortKey":"2","text":"dinner moved","timestamp":"2026-05-19T10:01:00Z","isSender":true,"senderName":"Me"}
		  ],
		  "hasMore": false, "oldestCursor": "o", "newestCursor": "n"
		}`))
	})

	results, err := client.SearchMessages(context.Background(), "dinner")
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if gotQuery != "dinner" {
		t.Errorf("query param = %q, want dinner", gotQuery)
	}
	if gotLimit != "20" {
		t.Errorf("limit param = %q, want 20", gotLimit)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Message.ChatID != "chat-2" {
		t.Errorf("results[0].ChatID = %q, want newest chat-2 first", results[0].Message.ChatID)
	}
	if !results[0].Message.IsFromMe {
		t.Error("results[0].IsFromMe = false, want true")
	}
	if !strings.Contains(results[1].Message.Text, "Dinner") {
		t.Errorf("results[1].Text = %q, want mapped text", results[1].Message.Text)
	}
}

func TestListMessages_MapsReactions(t *testing.T) {
	const body = `{
  "items": [
    {"id":"m1","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"1","text":"hey","timestamp":"2026-05-19T10:00:00Z","isSender":false,"senderName":"Bob",
     "reactions":[
       {"id":"u2-thumb","participantID":"u2","reactionKey":"👍","emoji":true},
       {"id":"u3-smile","participantID":"u3","reactionKey":"smiling-face","emoji":false}
     ]}
  ],
  "hasMore": false, "oldestCursor": "o", "newestCursor": "n"
}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	msgs, err := client.ListMessages(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(msgs) != 1 || len(msgs[0].Reactions) != 2 {
		t.Fatalf("got %d messages with %v reactions, want 1 message with 2 reactions", len(msgs), msgs)
	}
	if r := msgs[0].Reactions[0]; r.Key != "👍" || !r.Emoji {
		t.Errorf("Reactions[0] = %+v, want emoji 👍", r)
	}
	if r := msgs[0].Reactions[1]; r.Key != "smiling-face" || r.Emoji {
		t.Errorf("Reactions[1] = %+v, want non-emoji smiling-face", r)
	}
}

func TestListMessages_DropsReactionEvents(t *testing.T) {
	const body = `{
  "items": [
    {"id":"m1","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"1","text":"hey","timestamp":"2026-05-19T10:00:00Z","isSender":false,"senderName":"Bob"},
    {"id":"r1","accountID":"acc","chatID":"chat-1","senderID":"u2","sortKey":"2","text":"{{sender}} reacted 👍","timestamp":"2026-05-19T10:01:00Z","isSender":false,"senderName":"Eve","type":"REACTION"}
  ],
  "hasMore": false, "oldestCursor": "o", "newestCursor": "n"
}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	msgs, err := client.ListMessages(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != "m1" {
		t.Errorf("msgs = %+v, want only the real message m1", msgs)
	}
}
