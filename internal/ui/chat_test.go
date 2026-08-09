package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/llm"
)

func chatModelForTest() Model {
	m := New(nil, nil).WithLLM(llm.New("http://127.0.0.1:0/v1", "test-model"))
	m.width = 80
	m.height = 24
	m.chatModel = "test-model"
	return m
}

func TestCycleTabEntersAndLeavesChat(t *testing.T) {
	m := chatModelForTest()
	m.tab = TabArchive
	m, _ = m.cycleTab(1)
	if m.tab != TabChat || m.mode != ModeChat {
		t.Fatalf("tab = %v mode = %v, want TabChat/ModeChat", m.tab, m.mode)
	}
	m, _ = m.cycleTab(1)
	if m.tab != TabInbox || m.mode != ModeList {
		t.Fatalf("tab = %v mode = %v, want TabInbox/ModeList", m.tab, m.mode)
	}
}

func TestSubmitChatInputStartsSession(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChatInsert
	m.chatInput = "who owes me money?"
	m, cmd := m.submitChatInput()
	if m.chatSession == nil {
		t.Fatal("no session started")
	}
	m.chatSession.cancel()
	if cmd == nil {
		t.Fatal("no wait command returned")
	}
	if len(m.chatTurns) != 2 {
		t.Fatalf("turns = %d, want you + assistant", len(m.chatTurns))
	}
	if m.chatTurns[0].role != chatYou || m.chatTurns[0].text != "who owes me money?" {
		t.Errorf("first turn = %+v", m.chatTurns[0])
	}
	if !m.chatTurns[1].streaming {
		t.Error("assistant turn not streaming")
	}
	if m.chatInput != "" || m.mode != ModeChat {
		t.Errorf("input = %q mode = %v after submit", m.chatInput, m.mode)
	}
}

func TestSubmitChatInputIgnoredWhileStreaming(t *testing.T) {
	m := chatModelForTest()
	m.chatInput = "first"
	m, _ = m.submitChatInput()
	defer m.chatSession.cancel()
	m.chatInput = "second"
	m2, _ := m.submitChatInput()
	if len(m2.chatTurns) != 2 {
		t.Fatalf("turns = %d, second submit should be ignored", len(m2.chatTurns))
	}
}

func TestApplyChatEventStreamsIntoLastTurn(t *testing.T) {
	m := chatModelForTest()
	m.chatInput = "q"
	m, _ = m.submitChatInput()
	defer m.chatSession.cancel()

	m, _ = m.applyChatEvent(chatEvent{kind: chatEvToolStart, step: toolStep{name: "search_messages", args: "cabin", running: true}})
	m, _ = m.applyChatEvent(chatEvent{kind: chatEvToolEnd, step: toolStep{name: "search_messages", args: "cabin", result: "2 results"}})
	m, _ = m.applyChatEvent(chatEvent{kind: chatEvToken, text: "Hel"})
	m, _ = m.applyChatEvent(chatEvent{kind: chatEvToken, text: "lo"})

	turn := m.chatTurns[len(m.chatTurns)-1]
	if turn.text != "Hello" {
		t.Errorf("text = %q", turn.text)
	}
	if len(turn.steps) != 1 || turn.steps[0].running || turn.steps[0].result != "2 results" {
		t.Errorf("steps = %+v", turn.steps)
	}
	if m.chatTokens != 2 {
		t.Errorf("tokens = %d", m.chatTokens)
	}

	m, _ = m.applyChatEvent(chatEvent{kind: chatEvDone})
	if m.chatSession != nil {
		t.Error("session not cleared on done")
	}
	if m.chatTurns[len(m.chatTurns)-1].streaming {
		t.Error("turn still streaming after done")
	}
}

func TestApplyChatEventError(t *testing.T) {
	m := chatModelForTest()
	m.chatInput = "q"
	m, _ = m.submitChatInput()
	defer func() {
		if m.chatSession != nil {
			m.chatSession.cancel()
		}
	}()
	m, _ = m.applyChatEvent(chatEvent{kind: chatEvErr, err: errors.New("connection refused")})
	turn := m.chatTurns[len(m.chatTurns)-1]
	if turn.errText == "" || turn.streaming {
		t.Errorf("turn = %+v, want error recorded", turn)
	}
	if m.chatSession != nil {
		t.Error("session not cleared on error")
	}
}

func TestCancelChatSessionKeepsPartialText(t *testing.T) {
	m := chatModelForTest()
	m.chatInput = "q"
	m, _ = m.submitChatInput()
	m, _ = m.applyChatEvent(chatEvent{kind: chatEvToken, text: "partial"})
	m = m.cancelChatSession()
	if m.chatSession != nil {
		t.Fatal("session not cleared")
	}
	turn := m.chatTurns[len(m.chatTurns)-1]
	if turn.streaming || turn.text != "partial" || turn.errText != "" {
		t.Errorf("turn = %+v", turn)
	}
}

func TestChatHistorySkipsErroredTurns(t *testing.T) {
	m := chatModelForTest()
	m.chatTurns = []chatTurn{
		{role: chatYou, text: "q1"},
		{role: chatAssistant, text: "a1"},
		{role: chatYou, text: "q2"},
		{role: chatAssistant, errText: "boom"},
	}
	h := m.chatHistory()
	if len(h) != 4 { // system + q1 + a1 + q2
		t.Fatalf("history = %d messages, want 4", len(h))
	}
	if h[0].Role != "system" || h[3].Content != "q2" {
		t.Errorf("history = %+v", h)
	}
}

func chatFixtureTurns() []chatTurn {
	return []chatTurn{
		{role: chatYou, text: "who owes me money?"},
		{role: chatAssistant, text: "Dana owes you $12.", steps: []toolStep{{
			name: "search_messages", args: "owe", result: "1 result",
			sources: []chatSource{{chatID: "stub-dana", chatTitle: "Dana Fixture", sender: "Dana Fixture", snippet: "you still owe me $12", ts: time.Now()}},
		}}},
	}
}

func TestChatLinesInlineSteps(t *testing.T) {
	m := chatModelForTest()
	m.chatTurns = chatFixtureTurns()
	joined := strings.Join(m.chatLines(), "\n")
	if !strings.Contains(joined, "⏺ search_messages(owe)") {
		t.Errorf("output missing inline step line:\n%s", joined)
	}
	if !strings.Contains(joined, "Dana owes you $12.") {
		t.Errorf("answer text missing:\n%s", joined)
	}
}

func TestHandleChatInsertKey(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m, _ = m.handleChatKey("i")
	if m.mode != ModeChatInsert {
		t.Fatalf("mode = %v after i", m.mode)
	}
	m, _ = m.handleChatInsertKey("h", "h")
	m, _ = m.handleChatInsertKey("i", "i")
	m, _ = m.handleChatInsertKey("backspace", "")
	if m.chatInput != "h" {
		t.Errorf("input = %q", m.chatInput)
	}
	m, _ = m.handleChatInsertKey("esc", "")
	if m.mode != ModeChat || m.chatInput != "" {
		t.Errorf("mode = %v input = %q after esc", m.mode, m.chatInput)
	}
}

func TestRenderChatNoLLM(t *testing.T) {
	m := New(nil, nil)
	m.width = 80
	m.height = 24
	m.mode = ModeChat
	m.tab = TabChat
	out := m.renderChat()
	if !strings.Contains(out, "No local model endpoint configured") {
		t.Errorf("missing setup help:\n%s", out)
	}
}
