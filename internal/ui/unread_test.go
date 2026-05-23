package ui

import (
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestSortChats_UnreadFloatToTopThenRecency(t *testing.T) {
	t0 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	chats := []api.Chat{
		{ID: "readOld", Unread: 0, LastActive: t0},
		{ID: "unreadOld", Unread: 2, LastActive: t0},
		{ID: "readNew", Unread: 0, LastActive: t0.Add(2 * time.Hour)},
		{ID: "unreadNew", Unread: 1, LastActive: t0.Add(time.Hour)},
	}
	sortChats(chats)
	got := []string{chats[0].ID, chats[1].ID, chats[2].ID, chats[3].ID}
	want := []string{"unreadNew", "unreadOld", "readNew", "readOld"}
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
