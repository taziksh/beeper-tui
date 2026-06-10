package ui

import (
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestTabIncludes(t *testing.T) {
	inbox := api.Chat{ID: "inbox", Unread: 0}
	unread := api.Chat{ID: "unread", Unread: 3}
	mention := api.Chat{ID: "mention", Mentions: 1}
	pinned := api.Chat{ID: "pinned", Pinned: true}
	muted := api.Chat{ID: "muted", Muted: true}
	archived := api.Chat{ID: "archived", Unread: 9, Archived: true}

	cases := []struct {
		tab  Tab
		chat api.Chat
		want bool
	}{
		{TabInbox, inbox, true},
		{TabInbox, muted, false},
		{TabInbox, archived, false},
		{TabUnread, unread, true},
		{TabUnread, inbox, false},
		{TabUnread, archived, false}, // archived never leaks into other tabs
		{TabMentions, mention, true},
		{TabPinned, pinned, true},
		{TabLowPriority, muted, true},
		{TabArchive, archived, true},
		{TabArchive, inbox, false},
	}
	for _, c := range cases {
		if got := c.tab.includes(c.chat); got != c.want {
			t.Errorf("%s.includes(%s) = %v, want %v", c.tab.label(), c.chat.ID, got, c.want)
		}
	}
}

func TestCycleTab_WrapsAndSelectsFirstChat(t *testing.T) {
	m := Model{
		mode: ModeList,
		tab:  TabInbox,
		chats: []api.Chat{
			{ID: "a"},
			{ID: "b", Unread: 2},
		},
		selected: 0,
	}
	m = m.cycleTab(1) // Inbox -> Unread
	if m.tab != TabUnread {
		t.Fatalf("tab = %v, want TabUnread", m.tab)
	}
	if m.chats[m.selected].ID != "b" {
		t.Errorf("selected = %q, want b (first chat in Unread)", m.chats[m.selected].ID)
	}
	m = m.cycleTab(-1) // back to Inbox
	if m.tab != TabInbox {
		t.Errorf("tab = %v, want TabInbox after reverse cycle", m.tab)
	}
}
