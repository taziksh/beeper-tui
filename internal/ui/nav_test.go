package ui

import (
	"errors"
	"strings"
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

func TestBackToList_ClearsConvErr(t *testing.T) {
	m := Model{mode: ModeConversation, currentChatID: "a", convErr: errors.New("read error")}
	m = m.backToList()
	if m.convErr != nil {
		t.Error("returning to the list should clear the conversation error")
	}
}

func TestHandleKey_Esc_AfterConvError_ReturnsToList(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24, currentChatID: "a",
		chats:   []api.Chat{{ID: "a", Title: "Alice"}, {ID: "b", Title: "Dev Team"}},
		convErr: errors.New("read error"),
	}
	m2, _ := m.handleKey("esc")
	if m2.mode != ModeList {
		t.Errorf("esc should return to the list, mode = %v", m2.mode)
	}
	out := m2.render()
	if strings.Contains(out, "read error") {
		t.Errorf("the error must not persist after returning to the list: %q", out)
	}
	if !strings.Contains(out, "Dev Team") {
		t.Errorf("list should be visible after esc: %q", out)
	}
}

func TestOpenSelected_ClearsStaleConvErr(t *testing.T) {
	m := Model{mode: ModeList, chats: []api.Chat{{ID: "a", Title: "Alice"}}, selected: 0, convErr: errors.New("old error")}
	m2, _ := m.openSelected()
	if m2.convErr != nil {
		t.Error("opening a chat should clear any stale conversation error")
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

func TestHandleKey_I_EntersInsertFromConversation(t *testing.T) {
	m := Model{mode: ModeConversation, currentChatID: "a"}
	m2, _ := m.handleKey("i")
	if m2.mode != ModeInsert {
		t.Errorf("mode = %v, want ModeInsert", m2.mode)
	}
}

func TestHandleKey_I_IgnoredInList(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(2)}
	m2, _ := m.handleKey("i")
	if m2.mode != ModeList {
		t.Errorf("mode = %v, want ModeList (i is a no-op in the list)", m2.mode)
	}
}

func TestHandleInsertKey_TypingAppendsText(t *testing.T) {
	m := Model{mode: ModeInsert}
	m, _ = m.handleInsertKey("h", "h")
	m, _ = m.handleInsertKey("i", "i")
	if m.input != "hi" {
		t.Errorf("input = %q, want hi", m.input)
	}
}

func TestHandleInsertKey_Backspace(t *testing.T) {
	m := Model{mode: ModeInsert, input: "hi"}
	m, _ = m.handleInsertKey("backspace", "")
	if m.input != "h" {
		t.Errorf("input = %q, want h", m.input)
	}
}

func TestHandleInsertKey_BackspaceEmpty_NoPanic(t *testing.T) {
	m := Model{mode: ModeInsert, input: ""}
	m, _ = m.handleInsertKey("backspace", "")
	if m.input != "" {
		t.Errorf("input = %q, want empty", m.input)
	}
}

func TestHandleInsertKey_EscDiscardsAndReturnsToNormal(t *testing.T) {
	m := Model{mode: ModeInsert, input: "draft", currentChatID: "a"}
	m, _ = m.handleInsertKey("esc", "")
	if m.mode != ModeConversation {
		t.Errorf("mode = %v, want ModeConversation", m.mode)
	}
	if m.input != "" {
		t.Errorf("input = %q, want empty (draft discarded)", m.input)
	}
}

func TestSendInput_AppendsOptimisticallyAndReturnsToNormal(t *testing.T) {
	m := Model{mode: ModeInsert, input: "see you at 7", currentChatID: "a", height: 24}
	m2, cmd := m.sendInput()
	if cmd == nil {
		t.Fatal("sendInput should return a send command")
	}
	if len(m2.messages) != 1 {
		t.Fatalf("messages = %d, want 1 optimistic message", len(m2.messages))
	}
	last := m2.messages[0]
	if last.Text != "see you at 7" || !last.IsFromMe {
		t.Errorf("optimistic message = %+v, want text 'see you at 7' from me", last)
	}
	if last.ID != "local:1" {
		t.Errorf("optimistic id = %q, want local:1", last.ID)
	}
	if m2.input != "" {
		t.Errorf("input = %q, want cleared", m2.input)
	}
	if m2.mode != ModeConversation {
		t.Errorf("mode = %v, want ModeConversation after send", m2.mode)
	}
}

func TestSendInput_EmptyIsNoOp(t *testing.T) {
	m := Model{mode: ModeInsert, input: "", currentChatID: "a"}
	m2, cmd := m.sendInput()
	if cmd != nil {
		t.Error("empty input must not issue a send command")
	}
	if len(m2.messages) != 0 {
		t.Error("empty input must not append a message")
	}
	if m2.mode != ModeInsert {
		t.Errorf("mode = %v, want ModeInsert (stay composing on empty enter)", m2.mode)
	}
}
