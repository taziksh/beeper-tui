package ui

import (
	tea "charm.land/bubbletea/v2"
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
