package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/state"
)

func warmCache() state.Cache {
	t0 := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	return state.Cache{
		LastSelectedChatID: "b",
		Chats: []api.Chat{
			{ID: "a", Title: "Alpha Test", Network: "Signal", LastActive: t0.Add(time.Hour)},
			{ID: "b", Title: "Beta Test", Network: "WhatsApp", LastActive: t0},
		},
	}
}

func TestWithCache_RendersChatsImmediately(t *testing.T) {
	m := New(nil, nil).WithCache(warmCache(), "/tmp/unused.json")
	if m.loadingChats {
		t.Error("loadingChats = true after warm start, want false")
	}
	m.width, m.height = 80, 20
	view := m.render()
	if !strings.Contains(view, "Alpha Test") || !strings.Contains(view, "Beta Test") {
		t.Errorf("first frame should show cached chats, got:\n%s", view)
	}
}

func TestWithCache_RestoresSelection(t *testing.T) {
	m := New(nil, nil).WithCache(warmCache(), "")
	if got := m.chats[m.selected].ID; got != "b" {
		t.Errorf("selected chat = %q, want b (LastSelectedChatID)", got)
	}
}

func TestWithCache_EmptyCacheStaysCold(t *testing.T) {
	m := New(nil, nil).WithCache(state.Cache{}, "/tmp/unused.json")
	if !m.loadingChats {
		t.Error("an empty cache must keep the loading screen")
	}
}

func TestWithCache_FreshFetchReplacesSnapshot(t *testing.T) {
	m := New(nil, nil).WithCache(warmCache(), "")
	got, _ := m.Update(chatsLoadedMsg{chats: []api.Chat{{ID: "c", Title: "Fresh Chat"}}})
	gm := got.(Model)
	if len(gm.chats) != 1 || gm.chats[0].Title != "Fresh Chat" {
		t.Errorf("chats = %+v, want the fresh fetch to replace the snapshot", gm.chats)
	}
}

func TestUpdate_ChatListErrorKeepsWarmStartList(t *testing.T) {
	m := New(nil, nil).WithCache(warmCache(), "")
	got, _ := m.Update(errMsg{err: errors.New("desktop not running")})
	gm := got.(Model)
	if gm.err != nil {
		t.Error("a failed refresh with cached chats on screen must not go full-screen")
	}
	if len(gm.chats) != 2 {
		t.Errorf("chats = %+v, want the cached list kept", gm.chats)
	}
}

func TestSnapshot_CapturesChatsAndSelection(t *testing.T) {
	m := New(nil, nil).WithCache(warmCache(), "")
	snap := m.Snapshot()
	if len(snap.Chats) != 2 {
		t.Fatalf("snapshot chats = %d, want 2", len(snap.Chats))
	}
	if snap.LastSelectedChatID != "b" {
		t.Errorf("LastSelectedChatID = %q, want b", snap.LastSelectedChatID)
	}
}

func TestMaybeSaveCache_WritesThenDebounces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	m := New(nil, nil).WithCache(warmCache(), path)

	m, cmd := m.maybeSaveCache()
	if cmd == nil {
		t.Fatal("first save should fire immediately")
	}
	cmd()
	loaded, err := state.Load(path)
	if err != nil || len(loaded.Chats) != 2 {
		t.Fatalf("Load() = %+v, %v; want the written snapshot back", loaded, err)
	}

	if _, cmd := m.maybeSaveCache(); cmd != nil {
		t.Error("a second save within the debounce window should be skipped")
	}
}

func TestMaybeSaveCache_DisabledWithoutPath(t *testing.T) {
	m := New(nil, nil).WithCache(warmCache(), "")
	if _, cmd := m.maybeSaveCache(); cmd != nil {
		t.Error("an empty cachePath must disable cache writes")
	}
}

func TestMaybeSaveCache_SkipsEmptyChatList(t *testing.T) {
	m := New(nil, nil).WithCache(state.Cache{}, filepath.Join(t.TempDir(), "cache.json"))
	if _, cmd := m.maybeSaveCache(); cmd != nil {
		t.Error("an empty chat list must not clobber a previous snapshot")
	}
}
