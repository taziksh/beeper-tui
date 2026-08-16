package ui

import (
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestResolveChatTitles(t *testing.T) {
	chats := []api.Chat{
		{ID: "c1", Type: "single", Title: "+1 (408) 750-7615"},
		{ID: "c2", Type: "single", Title: "(571) 977-9862"},
		{ID: "c3", Type: "single", Title: "39781"},
		{ID: "c4", Type: "single", Title: "Ada Testface"},
		{ID: "c5", Type: "group", Title: "555 0000"},
	}
	contacts := []api.Contact{
		{FullName: "Priya Blooming", PhoneNumber: "+14087507615"},
		{FullName: "Omar Fixture", PhoneNumber: "+15719779862"},
	}
	got := resolveChatTitles(chats, contacts)
	want := []string{"Priya Blooming", "Omar Fixture", "39781", "Ada Testface", "555 0000"}
	for i, w := range want {
		if got[i].Title != w {
			t.Errorf("chat %s title = %q, want %q", got[i].ID, got[i].Title, w)
		}
	}
	if chats[0].Title != "+1 (408) 750-7615" {
		t.Error("input slice mutated")
	}
}
