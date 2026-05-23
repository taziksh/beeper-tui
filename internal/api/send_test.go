package api_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSendMessage_PostsTextToChat(t *testing.T) {
	var gotPath, gotMethod, gotBody string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chatID":"chat-1","pendingMessageID":"pending-1"}`))
	})

	err := client.SendMessage(context.Background(), "chat-1", "hello there")
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.Contains(gotPath, "/v1/chats/chat-1/messages") {
		t.Errorf("path = %q, want it to contain /v1/chats/chat-1/messages", gotPath)
	}
	if !strings.Contains(gotBody, "hello there") {
		t.Errorf("body = %q, want it to contain the message text", gotBody)
	}
}

func TestSendMessage_EncodesHashInChatID(t *testing.T) {
	const chatID = "imsg##thread:abc123"
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chatID":"x","pendingMessageID":"p"}`))
	})

	if err := client.SendMessage(context.Background(), chatID, "hi"); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	want := "/v1/chats/" + chatID + "/messages"
	if gotPath != want {
		t.Errorf("server received path %q, want %q (a '#' chat id must survive encoding)", gotPath, want)
	}
}

func TestSendMessage_SurfacesError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})

	err := client.SendMessage(context.Background(), "chat-1", "hi")
	if err == nil {
		t.Fatal("SendMessage() error = nil, want non-nil on 500")
	}
}
