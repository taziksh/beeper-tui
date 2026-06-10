package api_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestArchiveChat_PostsArchiveRequest(t *testing.T) {
	var gotPath string
	var gotBody string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		gotBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.ArchiveChat(context.Background(), "chat-1", true); err != nil {
		t.Fatalf("ArchiveChat() error = %v", err)
	}
	if gotPath != "/v1/chats/chat-1/archive" {
		t.Errorf("path = %q, want /v1/chats/chat-1/archive", gotPath)
	}
	if !strings.Contains(gotBody, `"archived":true`) {
		t.Errorf("body = %q, want archived true", gotBody)
	}
}

func TestArchiveChat_EncodesHashInChatID(t *testing.T) {
	const chatID = "imsg##thread:abc123"
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.ArchiveChat(context.Background(), chatID, true); err != nil {
		t.Fatalf("ArchiveChat() error = %v", err)
	}
	want := "/v1/chats/" + chatID + "/archive"
	if gotPath != want {
		t.Errorf("server received path %q, want %q (a '#' chat id must survive encoding)", gotPath, want)
	}
}
