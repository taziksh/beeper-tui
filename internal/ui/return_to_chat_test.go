package ui

import "testing"

func TestReturnToChatViaQ(t *testing.T) {
	m := chatModelForTest()
	m.mode = ModeChat
	m.tab = TabChat
	m.chats = linkFixtureChats()
	m.chatLinks = []chatLink{{name: "Dana Fixture", chatID: "c-dana"}}
	m.chatLinkSel = 0
	m.chatTurns = []chatTurn{{role: chatAssistant, text: "talk to Dana Fixture"}}

	m, _ = m.handleChatKey("enter")
	if m.mode != ModeConversation {
		t.Fatalf("mode=%v want conversation", m.mode)
	}
	if !m.returnToChat {
		t.Fatal("returnToChat false after open")
	}
	m, cmd := m.handleKey("q")
	if cmd != nil {
		t.Fatalf("q should not quit, got cmd")
	}
	if m.mode != ModeChat {
		t.Fatalf("after q mode=%v want ModeChat", m.mode)
	}
	if m.tab != TabChat {
		t.Fatalf("tab=%v want TabChat", m.tab)
	}
	if len(m.chatTurns) != 1 {
		t.Fatalf("chatTurns wiped: %d", len(m.chatTurns))
	}
}
