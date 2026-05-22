package ui

import (
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func chats(n int) []api.Chat {
	cs := make([]api.Chat, n)
	for i := range cs {
		cs[i] = api.Chat{ID: string(rune('a' + i%26))}
	}
	return cs
}

func msgs(n int) []api.Message {
	ms := make([]api.Message, n)
	for i := range ms {
		ms[i] = api.Message{ID: string(rune('a' + i%26))}
	}
	return ms
}

func TestCursorDown_List_Advances(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(3), selected: 0, height: 10}
	m = m.cursorDown()
	if m.selected != 1 {
		t.Errorf("selected = %d, want 1", m.selected)
	}
}

func TestCursorDown_List_ClampsBottom(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(3), selected: 2, height: 10}
	m = m.cursorDown()
	if m.selected != 2 {
		t.Errorf("selected = %d, want clamped 2", m.selected)
	}
}

func TestCursorUp_List_ClampsTop(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(3), selected: 0, height: 10}
	m = m.cursorUp()
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0", m.selected)
	}
}

func TestCursorDown_EmptyList_NoPanic(t *testing.T) {
	m := Model{mode: ModeList, chats: nil, height: 10}
	m = m.cursorDown()
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0", m.selected)
	}
}

func TestJumpBottom_List(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(5), selected: 0, height: 10}
	m = m.jumpBottom()
	if m.selected != 4 {
		t.Errorf("selected = %d, want 4", m.selected)
	}
}

func TestJumpTop_List(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(5), selected: 4, offset: 3, height: 10}
	m = m.jumpTop()
	if m.selected != 0 || m.offset != 0 {
		t.Errorf("selected/offset = %d/%d, want 0/0", m.selected, m.offset)
	}
}

func TestOpenSelected_SwitchesMode(t *testing.T) {
	m := Model{mode: ModeList, chats: []api.Chat{{ID: "a", Title: "Alice"}}, selected: 0}
	m2, cmd := m.openSelected()
	if m2.mode != ModeConversation {
		t.Error("mode should be ModeConversation")
	}
	if m2.currentChatID != "a" {
		t.Errorf("currentChatID = %q, want a", m2.currentChatID)
	}
	if !m2.loadingMsgs {
		t.Error("loadingMsgs should be true while fetching")
	}
	if cmd == nil {
		t.Error("openSelected should return a command")
	}
}

func TestOpenSelected_EmptyList_NoOp(t *testing.T) {
	m := Model{mode: ModeList, chats: nil}
	m2, cmd := m.openSelected()
	if m2.mode != ModeList || cmd != nil {
		t.Error("opening with no chats should do nothing")
	}
}

func TestBackToList(t *testing.T) {
	m := Model{mode: ModeConversation, currentChatID: "a", messages: msgs(3)}
	m = m.backToList()
	if m.mode != ModeList {
		t.Error("mode should be ModeList")
	}
}

func TestConversationScroll_DownAndClamp(t *testing.T) {
	// height 7 -> visibleRows 5. 20 messages -> maxMsgOffset 15.
	m := Model{mode: ModeConversation, messages: msgs(20), height: 7}
	for i := 0; i < 100; i++ {
		m = m.cursorDown()
	}
	if m.msgOffset != 15 {
		t.Errorf("msgOffset = %d, want clamped 15", m.msgOffset)
	}
}

func TestConversationScroll_JumpTopBottom(t *testing.T) {
	m := Model{mode: ModeConversation, messages: msgs(20), height: 7, msgOffset: 10}
	m = m.jumpTop()
	if m.msgOffset != 0 {
		t.Errorf("after jumpTop msgOffset = %d, want 0", m.msgOffset)
	}
	m = m.jumpBottom()
	if m.msgOffset != 15 {
		t.Errorf("after jumpBottom msgOffset = %d, want 15", m.msgOffset)
	}
}
