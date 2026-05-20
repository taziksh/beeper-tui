package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestMarkRead_PostsToReadEndpoint(t *testing.T) {
	var gotPath, gotMethod string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat-1"}`))
	})

	err := client.MarkRead(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.Contains(gotPath, "/v1/chats/chat-1/read") {
		t.Errorf("path = %q, want it to contain /v1/chats/chat-1/read", gotPath)
	}
}
