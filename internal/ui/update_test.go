package ui

import (
	"errors"
	"fmt"
	"testing"
	"time"

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

func TestUpdate_MessagesLoaded_OpensAtBottom(t *testing.T) {
	// height 7 -> visibleRows 5; 20 messages -> maxMsgOffset 15.
	m := Model{currentChatID: "a", loadingMsgs: true, height: 7}
	got, _ := m.Update(messagesLoadedMsg{chatID: "a", messages: msgs(20)})
	gm := got.(Model)
	if gm.msgOffset != gm.maxMsgOffset() {
		t.Errorf("msgOffset = %d, want maxMsgOffset %d (open at bottom)", gm.msgOffset, gm.maxMsgOffset())
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

func TestUpdate_ConversationLoadError_ScopedNotGlobal(t *testing.T) {
	m := Model{mode: ModeConversation, currentChatID: "a", loadingMsgs: true}
	got, _ := m.Update(errMsg{chatID: "a", err: errors.New("read error")})
	gm := got.(Model)
	if gm.convErr == nil {
		t.Error("a conversation-load error should set convErr (scoped to the conversation)")
	}
	if gm.err != nil {
		t.Error("a conversation-load error must NOT set the global err (that traps the user full-screen)")
	}
	if gm.loadingMsgs {
		t.Error("loadingMsgs should be cleared after a conversation-load error")
	}
}

func TestUpdate_StaleConversationError_Ignored(t *testing.T) {
	m := Model{mode: ModeConversation, currentChatID: "a"}
	got, _ := m.Update(errMsg{chatID: "OLD", err: errors.New("read error")})
	gm := got.(Model)
	if gm.convErr != nil {
		t.Error("an error for a non-current chat must be ignored")
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

func TestUpdate_ChatsLoaded_SortsUnreadToTopAndPinsSelection(t *testing.T) {
	t0 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	// User had "b" selected before the refresh.
	m := Model{loadingChats: true, chats: []api.Chat{{ID: "a"}, {ID: "b"}}, selected: 1}
	got, _ := m.Update(chatsLoadedMsg{chats: []api.Chat{
		{ID: "a", Unread: 0, LastActive: t0.Add(time.Hour)},
		{ID: "b", Unread: 3, LastActive: t0},
	}})
	gm := got.(Model)
	if gm.chats[0].ID != "b" {
		t.Errorf("chats[0].ID = %q, want b (unread floats up)", gm.chats[0].ID)
	}
	if gm.chats[gm.selected].ID != "b" {
		t.Errorf("selection landed on %q, want b (pinned by ID across re-sort)", gm.chats[gm.selected].ID)
	}
}

func TestUpdate_MessagesLoaded_ScrollsToFirstUnread_Clamped(t *testing.T) {
	// height 7 -> visibleRows 5 -> maxMsgOffset 5. First unread at index 6 can't
	// sit at the very top, so the offset clamps to 5 (unread still visible).
	ms := make([]api.Message, 10)
	for i := range ms {
		ms[i] = api.Message{ID: fmt.Sprintf("m%d", i), Text: "x"}
	}
	ms[6].IsUnread = true
	ms[7].IsUnread = true
	m := Model{mode: ModeConversation, currentChatID: "a", loadingMsgs: true, height: 7}
	got, _ := m.Update(messagesLoadedMsg{chatID: "a", messages: ms})
	gm := got.(Model)
	if gm.msgOffset != 5 {
		t.Errorf("msgOffset = %d, want 5 (first unread clamped to keep it visible)", gm.msgOffset)
	}
}

func TestUpdate_MessagesLoaded_ScrollsToFirstUnread_MidList(t *testing.T) {
	// First unread at index 2, well within range, so the offset lands exactly on
	// it (no clamp): 2 <= maxMsgOffset 5.
	ms := make([]api.Message, 10)
	for i := range ms {
		ms[i] = api.Message{ID: fmt.Sprintf("m%d", i), Text: "x"}
	}
	ms[2].IsUnread = true
	m := Model{mode: ModeConversation, currentChatID: "a", loadingMsgs: true, height: 7}
	got, _ := m.Update(messagesLoadedMsg{chatID: "a", messages: ms})
	gm := got.(Model)
	if gm.msgOffset != 2 {
		t.Errorf("msgOffset = %d, want 2 (first unread at top, unclamped)", gm.msgOffset)
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
