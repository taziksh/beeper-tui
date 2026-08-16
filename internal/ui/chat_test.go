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
	m.chatChecked = true
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
	if !strings.Contains(turn.errText, "Can't reach model server") ||
		!strings.Contains(turn.errText, "Press enter to reconnect") {
		t.Errorf("error is not actionable: %q", turn.errText)
	}
	if m.chatErr == nil || !strings.Contains(m.chatStatusBar(), "offline") {
		t.Errorf("provider failure was not kept visible: err=%v status=%q", m.chatErr, m.chatStatusBar())
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
	if !strings.Contains(out, "Assistant isn't configured") {
		t.Errorf("missing setup help:\n%s", out)
	}
}

func TestRenderChatReadyStateShowsModelWithoutPitch(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m.tab = TabChat
	out := m.renderChat()
	if !strings.Contains(out, "model server · test-model") {
		t.Errorf("ready view missing model identity:\n%s", out)
	}
	for _, leftover := range []string{
		"Ask about your messages",
		"Search conversations",
		"Press enter to ask",
		"BEEPER_LLM",
		"Status    ",
		"Try",
		"?",
	} {
		if strings.Contains(out, leftover) {
			t.Errorf("ready view still contains %q:\n%s", leftover, out)
		}
	}
}

func TestRenderChatConnectingIsQuiet(t *testing.T) {
	m := New(nil, nil).WithLLM(llm.New("http://127.0.0.1:1234/v1", ""))
	m.width, m.height = 80, 24
	m.mode = ModeChat
	m.tab = TabChat
	m.chatDetecting = true
	out := m.renderChat()
	for _, want := range []string{"Connecting to LM Studio", "127.0.0.1:1234"} {
		if !strings.Contains(out, want) {
			t.Errorf("connecting view missing %q:\n%s", want, out)
		}
	}
	for _, leftover := range []string{"separate model server", "rest of beeper-tui works without it"} {
		if strings.Contains(out, leftover) {
			t.Errorf("connecting view still contains %q:\n%s", leftover, out)
		}
	}
}

func TestRenderChatShowsDesignedConnectionFailure(t *testing.T) {
	m := New(nil, nil).WithLLM(llm.New("http://127.0.0.1:1234/v1", ""))
	m.width, m.height = 80, 24
	m.mode = ModeChat
	m.tab = TabChat
	m.chatChecked = true
	m.chatErr = errors.New("dial tcp 127.0.0.1:1234: connect: connection refused")
	out := m.renderChat()
	for _, want := range []string{"Can't reach LM Studio", "Chat depends on LM Studio", "start Local Server", "Press enter to try again", "Command  lms server start", "Error    connection refused", "Server   LM Studio", "Config   BEEPER_LLM_BASE_URL", "NORMAL  assistant offline"} {
		if !strings.Contains(out, want) {
			t.Errorf("failure view missing %q:\n%s", want, out)
		}
	}
}

func TestRenderChatDistinguishesMissingModel(t *testing.T) {
	m := New(nil, nil).WithLLM(llm.New("http://127.0.0.1:1234/v1", ""))
	m.width, m.height = 80, 24
	m.mode, m.tab, m.chatChecked = ModeChat, TabChat, true
	m.chatErr = errors.New("no chat model loaded")
	out := m.renderChat()
	for _, want := range []string{"No chat model is available", "LM Studio is responding", "Load a chat model", "BEEPER_LLM_MODEL"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing-model view missing %q:\n%s", want, out)
		}
	}
}

func TestChatUnavailableStateWrapsToTerminalWidth(t *testing.T) {
	m := New(nil, nil).WithLLM(llm.New("http://127.0.0.1:1234/v1", ""))
	m.width, m.height = 50, 30
	m.mode, m.tab, m.chatChecked = ModeChat, TabChat, true
	m.chatErr = errors.New("dial tcp 127.0.0.1:1234: connect: connection refused")
	for _, line := range m.chatLines() {
		if len([]rune(line)) > m.width {
			t.Errorf("line exceeds width %d: %q", m.width, line)
		}
	}
}

func TestChatStatusBarIsTerseWhenReady(t *testing.T) {
	m := chatModelForTest()
	if got := m.chatStatusBar(); got != "NORMAL" {
		t.Errorf("chatStatusBar() = %q, want NORMAL", got)
	}
}

func TestChatServerNamesKnownLocalRuntimes(t *testing.T) {
	for _, tc := range []struct {
		url, name, address string
	}{
		{"http://127.0.0.1:1234/v1", "LM Studio", "127.0.0.1:1234"},
		{"http://localhost:11434/v1", "Ollama", "localhost:11434"},
		{"https://inference.tinfoil.sh/v1", "Tinfoil", "inference.tinfoil.sh"},
		{"http://127.0.0.1:9999/openai/v1", "model server", "127.0.0.1:9999/openai/v1"},
	} {
		m := New(nil, nil).WithLLM(llm.New(tc.url, ""))
		got := m.chatServer()
		if got.name != tc.name || got.address != tc.address {
			t.Errorf("chatServer(%q) = %+v, want %s at %s", tc.url, got, tc.name, tc.address)
		}
	}
}

func TestChatFailureBucketsForRemoteErrors(t *testing.T) {
	server := chatServerInfo{name: "Tinfoil", address: "inference.tinfoil.sh"}
	for _, tc := range []struct {
		err   string
		title string
	}{
		{"tinfoil: enclave attestation failed, nothing was sent: bad measurement", "Enclave attestation failed"},
		{"chat: HTTP 401: invalid api key", "Tinfoil rejected the API key"},
		{"chat: HTTP 429: slow down", "Rate limited"},
	} {
		got := chatFailureFor(errors.New(tc.err), server)
		if got.title != tc.title {
			t.Errorf("chatFailureFor(%q).title = %q, want %q", tc.err, got.title, tc.title)
		}
	}
}

func TestEnterRetriesWhenChatIsOffline(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m.chatErr = errors.New("connection refused")
	m, cmd := m.handleChatKey("enter")
	if cmd == nil || !m.chatDetecting || m.chatChecked || m.mode != ModeChat {
		t.Errorf("enter retry state = mode %v detecting %t checked %t cmd nil %t", m.mode, m.chatDetecting, m.chatChecked, cmd == nil)
	}
}

func TestAskKeyAlsoRetriesWhenChatIsOffline(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m.chatErr = errors.New("connection refused")
	m, cmd := m.handleChatKey("i")
	if cmd == nil || !m.chatDetecting || m.chatChecked || m.mode != ModeChat {
		t.Errorf("ask retry state = mode %v detecting %t checked %t cmd nil %t", m.mode, m.chatDetecting, m.chatChecked, cmd == nil)
	}
}

func TestEnterDoesNotOpenInputWhileConnecting(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m.chatChecked = false
	m.chatDetecting = true
	m, cmd := m.handleChatKey("enter")
	if cmd != nil || m.mode != ModeChat {
		t.Errorf("enter while connecting = mode %v cmd nil %t", m.mode, cmd == nil)
	}
}

func TestRetryChatEndpointClearsFailureAndChecksAgain(t *testing.T) {
	m := chatModelForTest()
	m.chatChecked = true
	m.chatErr = errors.New("connection refused")
	m, cmd := m.handleChatKey("r")
	if cmd == nil || !m.chatDetecting || m.chatChecked || m.chatErr != nil {
		t.Errorf("retry state = detecting %t checked %t err %v cmd nil %t", m.chatDetecting, m.chatChecked, m.chatErr, cmd == nil)
	}
}
