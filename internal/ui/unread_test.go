package ui

import (
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestSortChats_PinnedFirstThenRecency(t *testing.T) {
	t0 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	chats := []api.Chat{
		{ID: "old", LastActive: t0},
		{ID: "pinnedOld", Pinned: true, LastActive: t0},
		{ID: "new", LastActive: t0.Add(2 * time.Hour)},
		{ID: "pinnedNew", Pinned: true, LastActive: t0.Add(time.Hour)},
	}
	sortChats(chats)
	got := []string{chats[0].ID, chats[1].ID, chats[2].ID, chats[3].ID}
	want := []string{"pinnedNew", "pinnedOld", "new", "old"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortChats order = %v, want %v", got, want)
		}
	}
}

func TestReselectByID_FindsMovedChat(t *testing.T) {
	chats := []api.Chat{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if got := reselectByID(chats, "c"); got != 2 {
		t.Errorf("reselectByID = %d, want 2", got)
	}
}

func TestReselectByID_MissingFallsBackToZero(t *testing.T) {
	chats := []api.Chat{{ID: "a"}, {ID: "b"}}
	if got := reselectByID(chats, "gone"); got != 0 {
		t.Errorf("reselectByID = %d, want 0 (fallback)", got)
	}
}
