package ui

import (
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestApplyPollTick_RefreshesListAndOpenConversation(t *testing.T) {
	m := Model{mode: ModeConversation, currentChatID: "a"}
	_, cmd := m.applyPollTick()
	if cmd == nil {
		t.Fatal("poll tick must fire refresh commands and reschedule")
	}
}

func TestApplyMessagesRefreshed_PreservesScrollPosition(t *testing.T) {
	m := Model{
		mode:          ModeConversation,
		currentChatID: "a",
		messages:      msgs(20),
		msgOffset:     3,
		height:        7,
	}
	m = m.applyMessagesRefreshed(messagesRefreshedMsg{chatID: "a", messages: msgs(21)})
	if m.msgOffset != 3 {
		t.Errorf("msgOffset = %d, want 3; a background poll must not move the reader", m.msgOffset)
	}
	if len(m.messages) != 21 {
		t.Errorf("messages = %d, want refreshed to 21", len(m.messages))
	}
}

func TestApplyMessagesRefreshed_SticksToBottom(t *testing.T) {
	m := Model{
		mode:          ModeConversation,
		currentChatID: "a",
		messages:      msgs(20),
		height:        7,
	}
	m.msgOffset = m.maxMsgOffset()
	m = m.applyMessagesRefreshed(messagesRefreshedMsg{chatID: "a", messages: msgs(25)})
	if m.msgOffset != m.maxMsgOffset() {
		t.Errorf("msgOffset = %d, want pinned to new bottom %d", m.msgOffset, m.maxMsgOffset())
	}
}

func TestApplyMessagesRefreshed_KeepsPendingOptimisticSends(t *testing.T) {
	m := Model{
		mode:          ModeConversation,
		currentChatID: "a",
		messages: append(msgs(2), api.Message{
			ID: "local:1", ChatID: "a", Text: "still sending", IsFromMe: true,
		}),
		height: 7,
	}
	m = m.applyMessagesRefreshed(messagesRefreshedMsg{chatID: "a", messages: msgs(2)})
	if len(m.messages) != 3 || m.messages[2].ID != "local:1" {
		t.Errorf("messages = %+v, want the unconfirmed optimistic send kept", m.messages)
	}
}

func TestApplyMessagesRefreshed_DropsConfirmedOptimisticSends(t *testing.T) {
	m := Model{
		mode:          ModeConversation,
		currentChatID: "a",
		messages: []api.Message{
			{ID: "local:1", ChatID: "a", Text: "hi", IsFromMe: true},
		},
		height: 7,
	}
	refreshed := []api.Message{{ID: "srv1", ChatID: "a", Text: "hi", IsFromMe: true}}
	m = m.applyMessagesRefreshed(messagesRefreshedMsg{chatID: "a", messages: refreshed})
	if len(m.messages) != 1 || m.messages[0].ID != "srv1" {
		t.Errorf("messages = %+v, want the server copy only", m.messages)
	}
}

func TestApplyMessagesRefreshed_IgnoresStaleChat(t *testing.T) {
	m := Model{mode: ModeConversation, currentChatID: "a", messages: msgs(2)}
	m = m.applyMessagesRefreshed(messagesRefreshedMsg{chatID: "other", messages: msgs(9)})
	if len(m.messages) != 2 {
		t.Error("a refresh for a non-current chat must be ignored")
	}
}
