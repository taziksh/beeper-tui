package ui

import (
	"fmt"
	"strings"
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
	case ModeList, ModeSearch:
		indexes := m.visibleChatIndexes()
		pos := m.selectedVisibleChatPos(indexes)
		if pos >= 0 && pos < len(indexes)-1 {
			m.selected = indexes[pos+1]
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
	case ModeList, ModeSearch:
		indexes := m.visibleChatIndexes()
		pos := m.selectedVisibleChatPos(indexes)
		if pos > 0 {
			m.selected = indexes[pos-1]
		}
	case ModeConversation:
		if m.msgOffset > 0 {
			m.msgOffset--
		}
	}
	return m.clampWindow()
}

// halfPage scrolls a half screen at a time, vim-style (Ctrl-u up, Ctrl-d down):
// dir is -1 for up, +1 for down. In the list it moves the selection; in a
// conversation it moves the scroll offset. clampWindow keeps everything in range.
func (m Model) halfPage(dir int) Model {
	step := m.visibleRows() / 2
	if step < 1 {
		step = 1
	}
	switch m.mode {
	case ModeList, ModeSearch:
		indexes := m.visibleChatIndexes()
		if len(indexes) == 0 {
			return m
		}
		pos := m.selectedVisibleChatPos(indexes)
		if pos < 0 {
			pos = 0
		}
		pos += dir * step
		if pos < 0 {
			pos = 0
		}
		if pos > len(indexes)-1 {
			pos = len(indexes) - 1
		}
		m.selected = indexes[pos]
	case ModeConversation:
		m.msgOffset += dir * step
	}
	return m.clampWindow()
}

func (m Model) jumpTop() Model {
	switch m.mode {
	case ModeList, ModeSearch:
		if indexes := m.visibleChatIndexes(); len(indexes) > 0 {
			m.selected = indexes[0]
		}
	case ModeConversation:
		m.msgOffset = 0
	}
	return m.clampWindow()
}

func (m Model) jumpBottom() Model {
	switch m.mode {
	case ModeList, ModeSearch:
		if indexes := m.visibleChatIndexes(); len(indexes) > 0 {
			m.selected = indexes[len(indexes)-1]
		}
	case ModeConversation:
		m.msgOffset = m.maxMsgOffset()
	}
	return m.clampWindow()
}

// clampWindow keeps the active cursor visible within the scroll window.
func (m Model) clampWindow() Model {
	switch m.mode {
	case ModeList, ModeSearch:
		vr := m.visibleRows()
		indexes := m.visibleChatIndexes()
		pos := m.selectedVisibleChatPos(indexes)
		if pos < 0 {
			m.offset = 0
			return m
		}
		if pos < m.offset {
			m.offset = pos
		}
		if pos >= m.offset+vr {
			m.offset = pos - vr + 1
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

func (m Model) visibleChatIndexes() []int {
	if m.mode != ModeSearch || m.searchQuery == "" {
		indexes := make([]int, len(m.chats))
		for i := range m.chats {
			indexes[i] = i
		}
		return indexes
	}
	q := strings.ToLower(m.searchQuery)
	indexes := make([]int, 0, len(m.chats))
	for i, c := range m.chats {
		if strings.Contains(strings.ToLower(c.Title), q) || strings.Contains(strings.ToLower(c.Network), q) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (m Model) selectedVisibleChatPos(indexes []int) int {
	for pos, idx := range indexes {
		if idx == m.selected {
			return pos
		}
	}
	return -1
}

func (m Model) selectFirstVisibleChat() Model {
	indexes := m.visibleChatIndexes()
	if len(indexes) == 0 {
		m.offset = 0
		return m
	}
	if m.selectedVisibleChatPos(indexes) < 0 {
		m.selected = indexes[0]
	}
	return m.clampWindow()
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
	m.convErr = nil
	m.loadingMsgs = true
	return m, tea.Batch(m.loadMessagesCmd(chat.ID), m.markReadCmd(chat.ID))
}

func (m Model) backToList() Model {
	m.mode = ModeList
	m.convErr = nil
	m.searchQuery = ""
	return m
}

func (m Model) startSearch() Model {
	m.mode = ModeSearch
	m.searchQuery = ""
	m.offset = 0
	return m.selectFirstVisibleChat()
}

func (m Model) handleSearchKey(key, text string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.mode = ModeList
		m.searchQuery = ""
		m.offset = 0
		return m.clampWindow(), nil
	case "enter":
		return m.openSelected()
	case "backspace":
		if r := []rune(m.searchQuery); len(r) > 0 {
			m.searchQuery = string(r[:len(r)-1])
		}
		return m.selectFirstVisibleChat(), nil
	case "j", "down":
		return m.cursorDown(), nil
	case "k", "up":
		return m.cursorUp(), nil
	case "ctrl+d":
		return m.halfPage(1), nil
	case "ctrl+u":
		return m.halfPage(-1), nil
	case "G":
		return m.jumpBottom(), nil
	default:
		m.searchQuery += text
		return m.selectFirstVisibleChat(), nil
	}
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
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		if m.mode == ModeConversation {
			return m.backToList(), nil
		}
		return m, tea.Quit
	case "/":
		return m.startSearch(), nil
	case "j", "down":
		return m.cursorDown(), nil
	case "k", "up":
		return m.cursorUp(), nil
	case "ctrl+d":
		return m.halfPage(1), nil
	case "ctrl+u":
		return m.halfPage(-1), nil
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
	}
	return m, nil
}
