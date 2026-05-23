package ui

import (
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestRender_LoadingChats(t *testing.T) {
	m := Model{mode: ModeList, loadingChats: true, width: 80, height: 24}
	if !strings.Contains(m.render(), "Loading") {
		t.Errorf("loading view missing 'Loading': %q", m.render())
	}
}

func TestRender_ListShowsTitles(t *testing.T) {
	m := Model{
		mode: ModeList, width: 80, height: 24,
		chats: []api.Chat{
			{ID: "a", Network: "Signal", Title: "Alice", Unread: 0},
			{ID: "b", Network: "WhatsApp", Title: "Dev Team", Unread: 5},
		},
		selected: 0,
	}
	out := m.render()
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Dev Team") {
		t.Errorf("list missing titles: %q", out)
	}
	if !strings.Contains(out, "5") {
		t.Errorf("list missing unread count: %q", out)
	}
}

func TestRender_ConversationShowsMessages(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages: []api.Message{
			{ID: "m1", SenderName: "Alice", Text: "hey there", IsFromMe: false},
			{ID: "m2", SenderName: "Me", Text: "hi back", IsFromMe: true},
		},
	}
	out := m.render()
	if !strings.Contains(out, "hey there") || !strings.Contains(out, "hi back") {
		t.Errorf("conversation missing message text: %q", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("conversation missing chat title: %q", out)
	}
}

func TestRender_InsertShowsComposeLine(t *testing.T) {
	m := Model{
		mode: ModeInsert, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages:      []api.Message{{ID: "m1", SenderName: "Alice", Text: "hi"}},
		input:         "see you at 7",
	}
	out := m.render()
	if !strings.Contains(out, "> see you at 7") {
		t.Errorf("INSERT view missing compose line with draft: %q", out)
	}
	if !strings.Contains(out, "INSERT") {
		t.Errorf("INSERT view missing INSERT in status bar: %q", out)
	}
}

func TestRender_NormalConversationHasNoComposeLine(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages:      []api.Message{{ID: "m1", SenderName: "Alice", Text: "hi"}},
		input:         "leftover",
	}
	out := m.render()
	if strings.Contains(out, "> leftover") {
		t.Errorf("NORMAL conversation must not show a compose line: %q", out)
	}
	if !strings.Contains(out, "NORMAL") {
		t.Errorf("NORMAL conversation missing NORMAL in status bar: %q", out)
	}
}

func TestRender_FailedSendMarker(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages:      []api.Message{{ID: "local:1", SenderName: "You", Text: "nope", IsFromMe: true}},
		failedSends:   map[string]bool{"local:1": true},
	}
	out := m.render()
	if !strings.Contains(out, "! send failed") {
		t.Errorf("failed optimistic message missing marker: %q", out)
	}
}

func TestRender_ConversationLoading(t *testing.T) {
	m := Model{
		mode: ModeConversation, loadingMsgs: true, width: 80, height: 24,
		chats: []api.Chat{{ID: "a", Title: "Alice"}}, currentChatID: "a",
	}
	if !strings.Contains(m.render(), "Loading") {
		t.Errorf("expected loading text: %q", m.render())
	}
}
