package ui

import (
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

func linkFixtureChats() []api.Chat {
	now := time.Now()
	return []api.Chat{
		{ID: "c-dana", Title: "Dana Fixture", Type: "single", LastActive: now},
		{ID: "c-group", Title: "Ski Trip", Type: "group", LastActive: now.Add(-time.Hour),
			Participants: []api.Participant{{UserID: "u-bob", FullName: "Bob Ramírez"}}},
		{ID: "c-bob", Title: "Bob Ramírez", Type: "single", LastActive: now.Add(-2 * time.Hour),
			Participants: []api.Participant{{UserID: "u-bob", FullName: "Bob Ramírez"}}},
	}
}

func TestFindChatLinks(t *testing.T) {
	text := "Dana Fixture said the Ski Trip is on; Bob Ramírez confirmed."
	links := findChatLinks(text, linkFixtureChats())
	if len(links) != 3 {
		t.Fatalf("links = %+v, want 3", links)
	}
	if links[0].chatID != "c-dana" || links[1].chatID != "c-group" {
		t.Errorf("order = %+v, want dana then ski trip", links)
	}
	// Bob appears in two chats; the link must pick the most recently active.
	if links[2].chatID != "c-group" && links[2].chatID != "c-bob" {
		t.Errorf("bob link = %+v", links[2])
	}
}

func TestFindChatLinksWordBoundary(t *testing.T) {
	links := findChatLinks("the danaher meeting", []api.Chat{
		{ID: "c-dana", Title: "Dana", Type: "single"},
	})
	if len(links) != 0 {
		t.Fatalf("substring inside a word linked: %+v", links)
	}
}

func TestChatLinkKeys(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m.tab = TabChat
	m.chats = linkFixtureChats()
	m.chatLinks = []chatLink{{name: "Dana Fixture", chatID: "c-dana"}, {name: "Ski Trip", chatID: "c-group"}}
	m.chatLinkSel = -1

	m, _ = m.handleChatKey("n")
	m, _ = m.handleChatKey("n")
	if m.chatLinkSel != 1 {
		t.Fatalf("sel = %d after two n presses, want 1", m.chatLinkSel)
	}
	m, _ = m.handleChatKey("N")
	if m.chatLinkSel != 0 {
		t.Fatalf("sel = %d after N, want 0", m.chatLinkSel)
	}
	// From no selection, N should land on the last link.
	m.chatLinkSel = -1
	m, _ = m.handleChatKey("N")
	if m.chatLinkSel != 1 {
		t.Fatalf("sel = %d from empty via N, want last (1)", m.chatLinkSel)
	}
	m, _ = m.handleChatKey("esc")
	if m.chatLinkSel != -1 {
		t.Fatalf("esc did not clear selection")
	}
	m, _ = m.handleChatKey("n")
	m, _ = m.handleChatKey("enter")
	if m.mode != ModeConversation || m.currentChatID != "c-dana" {
		t.Fatalf("enter on link: mode=%v chat=%s, want conversation c-dana", m.mode, m.currentChatID)
	}
	if !m.returnToChat {
		t.Fatal("returnToChat not set after opening a link from Chat")
	}
	// q should restore the Chat tab, not dump into the inbox list.
	m = m.backToList()
	if m.mode != ModeChat || m.tab != TabChat {
		t.Fatalf("after q: mode=%v tab=%v, want ModeChat/TabChat", m.mode, m.tab)
	}
	if m.returnToChat {
		t.Fatal("returnToChat should clear after back")
	}
}

func TestChatTabKeysAlwaysSwitchAppTabs(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m.tab = TabChat
	m.chatLinks = []chatLink{{name: "Dana Fixture", chatID: "c-dana"}}
	m.chatLinkSel = 0

	m, _ = m.handleChatKey("tab")
	if m.tab != TabInbox || m.mode != ModeList {
		t.Fatalf("tab = %v mode = %v after tab, want Inbox/List", m.tab, m.mode)
	}
	if m.chatLinkSel != -1 {
		t.Fatalf("tab should clear transient link selection, got %d", m.chatLinkSel)
	}

	m, _ = m.cycleTab(-1)
	if m.tab != TabChat || m.mode != ModeChat {
		t.Fatalf("tab = %v mode = %v after returning, want Chat/Chat", m.tab, m.mode)
	}
	m, _ = m.handleChatKey("shift+tab")
	if m.tab != TabArchive || m.mode != ModeList {
		t.Fatalf("tab = %v mode = %v after shift+tab, want Archive/List", m.tab, m.mode)
	}
}

func TestChatLinkKeysDoNothingWithoutLinks(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m.tab = TabChat
	m, _ = m.handleChatKey("n")
	m, _ = m.handleChatKey("N")
	if m.tab != TabChat || m.chatLinkSel != -1 {
		t.Fatalf("n/N without links changed state: tab=%v sel=%d", m.tab, m.chatLinkSel)
	}
}

func TestOpenSelectedDoesNotReturnToChat(t *testing.T) {
	// Normal list → conversation → q still goes to the list.
	m := chatModelForTest()
	m.mode = ModeList
	m.tab = TabInbox
	m.chats = linkFixtureChats()
	m.selected = 0
	m, _ = m.openSelected()
	if m.returnToChat {
		t.Fatal("returnToChat set for a normal open")
	}
	m = m.backToList()
	if m.mode != ModeList {
		t.Fatalf("mode = %v, want ModeList", m.mode)
	}
}

func TestChatEnterWithoutLinkStillInserts(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m, _ = m.handleChatKey("enter")
	if m.mode != ModeChatInsert {
		t.Fatalf("mode = %v, want ModeChatInsert", m.mode)
	}
}
