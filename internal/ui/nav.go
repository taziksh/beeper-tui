package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
)

// visibleRows is how many rows the list/conversation body can show. It reserves
// two rows (a header and the status bar). Falls back to a minimum of 1 before
// the first WindowSizeMsg sets height.
func (m Model) visibleRows() int {
	r := m.height - 2
	if r < 1 {
		return 1
	}
	return r
}

func (m Model) maxMsgOffset() int {
	max := len(m.messages) - m.visibleRows()
	if max < 0 {
		return 0
	}
	return max
}

func (m Model) cursorDown() Model {
	switch m.mode {
	case ModeList:
		if len(m.chats) > 0 && m.selected < len(m.chats)-1 {
			m.selected++
		}
	case ModeConversation:
		if m.msgOffset < m.maxMsgOffset() {
			m.msgOffset++
		}
	}
	return m.clampWindow()
}

func (m Model) cursorUp() Model {
	switch m.mode {
	case ModeList:
		if m.selected > 0 {
			m.selected--
		}
	case ModeConversation:
		if m.msgOffset > 0 {
			m.msgOffset--
		}
	}
	return m.clampWindow()
}

func (m Model) jumpTop() Model {
	switch m.mode {
	case ModeList:
		m.selected = 0
	case ModeConversation:
		m.msgOffset = 0
	}
	return m.clampWindow()
}

func (m Model) jumpBottom() Model {
	switch m.mode {
	case ModeList:
		if len(m.chats) > 0 {
			m.selected = len(m.chats) - 1
		}
	case ModeConversation:
		m.msgOffset = m.maxMsgOffset()
	}
	return m.clampWindow()
}

// clampWindow keeps the active cursor visible within the scroll window.
func (m Model) clampWindow() Model {
	switch m.mode {
	case ModeList:
		vr := m.visibleRows()
		if m.selected < m.offset {
			m.offset = m.selected
		}
		if m.selected >= m.offset+vr {
			m.offset = m.selected - vr + 1
		}
		if m.offset < 0 {
			m.offset = 0
		}
	case ModeConversation:
		if m.msgOffset > m.maxMsgOffset() {
			m.msgOffset = m.maxMsgOffset()
		}
		if m.msgOffset < 0 {
			m.msgOffset = 0
		}
	}
	return m
}

// openSelected enters the selected chat: switches mode, resets scroll, and
// returns a command that loads messages and marks the chat read.
func (m Model) openSelected() (Model, tea.Cmd) {
	if len(m.chats) == 0 || m.selected >= len(m.chats) {
		return m, nil
	}
	chat := m.chats[m.selected]
	m.mode = ModeConversation
	m.currentChatID = chat.ID
	m.messages = nil
	m.msgOffset = 0
	m.loadingMsgs = true
	return m, tea.Batch(m.loadMessagesCmd(chat.ID), m.markReadCmd(chat.ID))
}

func (m Model) backToList() Model {
	m.mode = ModeList
	return m
}

// handleInsertKey processes keys while composing. `key` is the key name
// (e.g. "enter", "esc", "backspace"); `text` is the literal characters a
// printable key produced (empty for named keys).
func (m Model) handleInsertKey(key, text string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.input = ""
		m.mode = ModeConversation
		return m, nil
	case "enter":
		return m.sendInput()
	case "backspace":
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
		return m, nil
	default:
		m.input += text
		return m, nil
	}
}

// sendInput appends the draft as an optimistic message, clears the compose
// line, returns to NORMAL, and fires the send command. Empty input is a no-op
// (stays in INSERT).
func (m Model) sendInput() (Model, tea.Cmd) {
	text := m.input
	if text == "" {
		return m, nil
	}
	m.localSeq++
	localID := fmt.Sprintf("local:%d", m.localSeq)
	m.messages = append(m.messages, api.Message{
		ID:         localID,
		ChatID:     m.currentChatID,
		SenderName: "You",
		Text:       text,
		Timestamp:  time.Now(),
		IsFromMe:   true,
	})
	m.input = ""
	m.mode = ModeConversation
	m = m.jumpBottom()
	return m, m.sendMessageCmd(m.currentChatID, localID, text)
}

// handleKey maps a key string to a pure method. Update stays thin.
func (m Model) handleKey(key string) (Model, tea.Cmd) {
	if key != "g" {
		m.pendingG = false
	}
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "j", "down":
		return m.cursorDown(), nil
	case "k", "up":
		return m.cursorUp(), nil
	case "G":
		return m.jumpBottom(), nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			return m.jumpTop(), nil
		}
		m.pendingG = true
		return m, nil
	case "i":
		if m.mode == ModeConversation {
			m.mode = ModeInsert
		}
		return m, nil
	case "enter":
		if m.mode == ModeList {
			return m.openSelected()
		}
		return m, nil
	case "esc":
		if m.mode == ModeConversation {
			return m.backToList(), nil
		}
		return m, nil
	}
	return m, nil
}
