package state_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/state"
)

func TestSave_WritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	c := state.Cache{
		SchemaVersion: state.CurrentSchemaVersion,
		LastSelectedChatID: "chat-123",
		Chats: []state.ChatSnapshot{
			{
				ID:        "chat-123",
				Name:      "Sarah Kim",
				Account:   "iMessage",
				Unread:    3,
				LastTs:    time.Date(2026, 5, 17, 10, 42, 0, 0, time.UTC),
				LastBody:  "hey did you see the article",
			},
		},
	}

	if err := state.Save(path, c); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var decoded state.Cache
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, raw: %s", err, raw)
	}
	if decoded.LastSelectedChatID != "chat-123" {
		t.Errorf("LastSelectedChatID = %q, want %q", decoded.LastSelectedChatID, "chat-123")
	}
	if len(decoded.Chats) != 1 {
		t.Fatalf("len(Chats) = %d, want 1", len(decoded.Chats))
	}
	if decoded.Chats[0].Name != "Sarah Kim" {
		t.Errorf("Chats[0].Name = %q, want %q", decoded.Chats[0].Name, "Sarah Kim")
	}
}

func TestLoad_RoundTripsSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	original := state.Cache{
		LastSelectedChatID: "abc",
		Chats: []state.ChatSnapshot{
			{ID: "abc", Name: "Test Chat", Account: "Signal", Unread: 1},
		},
	}
	if err := state.Save(path, original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.LastSelectedChatID != "abc" {
		t.Errorf("LastSelectedChatID = %q, want %q", got.LastSelectedChatID, "abc")
	}
	if len(got.Chats) != 1 || got.Chats[0].Name != "Test Chat" {
		t.Errorf("Chats = %+v, want one chat named Test Chat", got.Chats)
	}
	if got.SchemaVersion != state.CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", got.SchemaVersion, state.CurrentSchemaVersion)
	}
}
