package state_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/state"
)

func TestSave_WritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	c := state.Cache{
		SchemaVersion:      state.CurrentSchemaVersion,
		LastSelectedChatID: "chat-123",
		Chats: []api.Chat{
			{
				ID:         "chat-123",
				Network:    "iMessage",
				Title:      "Sarah Kim",
				Unread:     3,
				LastActive: time.Date(2026, 5, 17, 10, 42, 0, 0, time.UTC),
				Preview:    "hey did you see the article",
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
	if decoded.Chats[0].Title != "Sarah Kim" {
		t.Errorf("Chats[0].Title = %q, want %q", decoded.Chats[0].Title, "Sarah Kim")
	}
}

func TestLoad_RoundTripsSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	original := state.Cache{
		LastSelectedChatID: "abc",
		Chats: []api.Chat{
			{ID: "abc", Title: "Test Chat", Network: "Signal", Unread: 1, Muted: true, Pinned: true},
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
	if len(got.Chats) != 1 || got.Chats[0].Title != "Test Chat" {
		t.Errorf("Chats = %+v, want one chat titled Test Chat", got.Chats)
	}
	if !got.Chats[0].Muted || !got.Chats[0].Pinned {
		t.Errorf("Chats[0] flags = %+v, want Muted and Pinned preserved", got.Chats[0])
	}
	if got.SchemaVersion != state.CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", got.SchemaVersion, state.CurrentSchemaVersion)
	}
}

func TestLoad_MissingFileReturnsEmptyCacheNoError(t *testing.T) {
	got, err := state.Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for missing file", err)
	}
	if len(got.Chats) != 0 {
		t.Errorf("Chats = %+v, want empty", got.Chats)
	}
	if got.SchemaVersion != 0 {
		t.Errorf("SchemaVersion = %d, want 0 for missing file", got.SchemaVersion)
	}
}

func TestLoad_CorruptJSONReturnsSentinel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o600); err != nil {
		t.Fatalf("setup WriteFile() error = %v", err)
	}

	_, err := state.Load(path)
	if !errors.Is(err, state.ErrCorruptCache) {
		t.Errorf("Load() error = %v, want errors.Is(err, ErrCorruptCache)", err)
	}
}

func TestLoad_SchemaMismatchReturnsSentinel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	// A v1 cache file from before the api.Chat snapshot must read as a schema
	// mismatch, not as corrupt or valid.
	body := []byte(`{"schema_version": 1, "last_selected_chat_id": "x", "chats": []}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("setup WriteFile() error = %v", err)
	}

	_, err := state.Load(path)
	if !errors.Is(err, state.ErrSchemaMismatch) {
		t.Errorf("Load() error = %v, want errors.Is(err, ErrSchemaMismatch)", err)
	}
}
