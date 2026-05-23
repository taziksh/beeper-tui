package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestUpdate_ChatsLoaded(t *testing.T) {
	m := Model{loadingChats: true}
	got, _ := m.Update(chatsLoadedMsg{chats: []api.Chat{{ID: "a", Title: "Alice"}}})
	gm := got.(Model)
	if gm.loadingChats {
		t.Error("loadingChats should be false")
	}
	if len(gm.chats) != 1 || gm.chats[0].Title != "Alice" {
		t.Errorf("chats = %+v, want one 'Alice'", gm.chats)
	}
}

func TestUpdate_MessagesLoadedForCurrentChat(t *testing.T) {
	m := Model{currentChatID: "a", loadingMsgs: true}
	got, _ := m.Update(messagesLoadedMsg{chatID: "a", messages: []api.Message{{ID: "m1", Text: "hi"}}})
	gm := got.(Model)
	if gm.loadingMsgs {
		t.Error("loadingMsgs should be false")
	}
	if len(gm.messages) != 1 || gm.messages[0].Text != "hi" {
		t.Errorf("messages = %+v, want one 'hi'", gm.messages)
	}
}

func TestUpdate_MessagesIgnoredForStaleChat(t *testing.T) {
	m := Model{currentChatID: "a", loadingMsgs: true}
	got, _ := m.Update(messagesLoadedMsg{chatID: "OLD", messages: []api.Message{{ID: "x"}}})
	gm := got.(Model)
	if len(gm.messages) != 0 {
		t.Error("messages for a non-current chat must be ignored")
	}
}

func TestUpdate_Error(t *testing.T) {
	m := Model{loadingChats: true}
	got, _ := m.Update(errMsg{err: errors.New("boom")})
	gm := got.(Model)
	if gm.err == nil || gm.loadingChats {
		t.Error("error should be set and loading cleared")
	}
}

func TestUpdate_SendResultError_MarksFailed(t *testing.T) {
	m := Model{failedSends: map[string]bool{}}
	got, _ := m.Update(sendResultMsg{localID: "local:1", err: errors.New("network down")})
	gm := got.(Model)
	if !gm.failedSends["local:1"] {
		t.Error("failedSends[local:1] should be true after an errored send")
	}
}

func TestUpdate_SendResultSuccess_NotMarked(t *testing.T) {
	m := Model{failedSends: map[string]bool{}}
	got, _ := m.Update(sendResultMsg{localID: "local:1", err: nil})
	gm := got.(Model)
	if gm.failedSends["local:1"] {
		t.Error("a successful send must not be marked failed")
	}
}

func TestUpdate_WindowSizeReclamps(t *testing.T) {
	m := Model{mode: ModeConversation, messages: msgs(50), msgOffset: 40, height: 30}
	got, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 6})
	gm := got.(Model)
	if gm.height != 6 {
		t.Errorf("height = %d, want 6", gm.height)
	}
	if gm.msgOffset > gm.maxMsgOffset() {
		t.Errorf("msgOffset %d exceeds max %d after resize", gm.msgOffset, gm.maxMsgOffset())
	}
}
