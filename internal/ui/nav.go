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
	case ModeList:
		indexes := m.visibleChatIndexes()
		pos := m.selectedVisibleChatPos(indexes)
		if pos >= 0 && pos < len(indexes)-1 {
			m.selected = indexes[pos+1]
		}
	case ModeSearch:
		if m.searchSelected < len(m.searchResults)-1 {
			m.searchSelected++
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
		indexes := m.visibleChatIndexes()
		pos := m.selectedVisibleChatPos(indexes)
		if pos > 0 {
			m.selected = indexes[pos-1]
		}
	case ModeSearch:
		if m.searchSelected > 0 {
			m.searchSelected--
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
	case ModeList:
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
	case ModeSearch:
		if len(m.searchResults) == 0 {
			return m
		}
		m.searchSelected += dir * step
		if m.searchSelected < 0 {
			m.searchSelected = 0
		}
		if m.searchSelected > len(m.searchResults)-1 {
			m.searchSelected = len(m.searchResults) - 1
		}
	case ModeConversation:
		m.msgOffset += dir * step
	}
	return m.clampWindow()
}

func (m Model) jumpTop() Model {
	switch m.mode {
	case ModeList:
		if indexes := m.visibleChatIndexes(); len(indexes) > 0 {
			m.selected = indexes[0]
		}
	case ModeSearch:
		m.searchSelected = 0
	case ModeConversation:
		m.msgOffset = 0
	}
	return m.clampWindow()
}

func (m Model) jumpBottom() Model {
	switch m.mode {
	case ModeList:
		if indexes := m.visibleChatIndexes(); len(indexes) > 0 {
			m.selected = indexes[len(indexes)-1]
		}
	case ModeSearch:
		if len(m.searchResults) > 0 {
			m.searchSelected = len(m.searchResults) - 1
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
	case ModeSearch:
		vr := m.visibleRows()
		if len(m.searchResults) == 0 {
			m.searchSelected = 0
			m.searchOffset = 0
			return m
		}
		if m.searchSelected < 0 {
			m.searchSelected = 0
		}
		if m.searchSelected > len(m.searchResults)-1 {
			m.searchSelected = len(m.searchResults) - 1
		}
		if m.searchSelected < m.searchOffset {
			m.searchOffset = m.searchSelected
		}
		if m.searchSelected >= m.searchOffset+vr {
			m.searchOffset = m.searchSelected - vr + 1
		}
		if m.searchOffset < 0 {
			m.searchOffset = 0
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
	if m.mode == ModeSearch && m.searchQuery != "" {
		q := strings.ToLower(m.searchQuery)
		indexes := make([]int, 0, len(m.chats))
		for i, c := range m.chats {
			if strings.Contains(strings.ToLower(c.Title), q) || strings.Contains(strings.ToLower(c.Network), q) {
				indexes = append(indexes, i)
			}
		}
		return indexes
	}
	indexes := make([]int, 0, len(m.chats))
	for i, c := range m.chats {
		if m.tab.includes(c) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

// cycleTab switches to the next or previous tab and selects its first chat.
func (m Model) cycleTab(dir int) Model {
	cur := 0
	for i, t := range tabOrder {
		if t == m.tab {
			cur = i
			break
		}
	}
	m.tab = tabOrder[(cur+dir+len(tabOrder))%len(tabOrder)]
	m.offset = 0
	if indexes := m.visibleChatIndexes(); len(indexes) > 0 {
		m.selected = indexes[0]
	}
	return m.clampWindow()
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
	m.archiveErr = nil
	return m, tea.Batch(m.loadMessagesCmd(chat.ID), m.markReadCmd(chat.ID))
}

func (m Model) backToList() Model {
	m.mode = ModeList
	m.convErr = nil
	m.archiveErr = nil
	m.searchQuery = ""
	m.searchResults = nil
	m.searchErr = nil
	m.searchLoading = false
	return m
}

// archiveSelected toggles the archived state of the selected chat.
func (m Model) archiveSelected() (Model, tea.Cmd) {
	var chatID string
	var archived bool
	switch m.mode {
	case ModeList:
		idx := m.selected
		if idx < 0 || idx >= len(m.chats) {
			return m, nil
		}
		chatID = m.chats[idx].ID
		archived = m.chats[idx].Archived
	case ModeConversation:
		chatID = m.currentChatID
		if idx := chatIndexByID(m.chats, chatID); idx >= 0 {
			archived = m.chats[idx].Archived
		}
	default:
		return m, nil
	}
	if chatID == "" || m.archivingChatID != "" {
		return m, nil
	}
	m.archivingChatID = chatID
	m.archiveErr = nil
	return m, m.archiveChatCmd(chatID, !archived)
}

// applyArchive sets a chat's archived flag, moving it between the Archive tab and
// the inbox. Archiving the open conversation returns to the list.
func (m Model) applyArchive(chatID string, archived bool) Model {
	if idx := chatIndexByID(m.chats, chatID); idx >= 0 {
		m.chats[idx].Archived = archived
	}
	if archived && m.mode == ModeConversation && m.currentChatID == chatID {
		m.mode = ModeList
		m.currentChatID = ""
		m.messages = nil
		m.msgOffset = 0
		m.loadingMsgs = false
		m.convErr = nil
	}
	return m.selectFirstVisibleChat()
}

func (m Model) startSearch() Model {
	m.mode = ModeSearch
	m.searchQuery = ""
	m.searchResults = nil
	m.searchSelected = 0
	m.searchOffset = 0
	m.searchLoading = false
	m.searchErr = nil
	return m
}

func (m Model) handleSearchKey(key, text string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.mode = ModeList
		m.searchQuery = ""
		m.searchResults = nil
		m.searchSelected = 0
		m.searchOffset = 0
		m.searchLoading = false
		m.searchErr = nil
		return m.clampWindow(), nil
	case "enter":
		return m.openSelectedSearchResult()
	case "backspace":
		if r := []rune(m.searchQuery); len(r) > 0 {
			m.searchQuery = string(r[:len(r)-1])
		}
		return m.searchAfterQueryChange()
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
		if text == "" {
			return m, nil
		}
		m.searchQuery += text
		return m.searchAfterQueryChange()
	}
}

func (m Model) searchAfterQueryChange() (Model, tea.Cmd) {
	m.searchResults = nil
	m.searchSelected = 0
	m.searchOffset = 0
	m.searchErr = nil
	query := strings.TrimSpace(m.searchQuery)
	if query == "" {
		m.searchLoading = false
		return m, nil
	}
	m.searchLoading = true
	return m, m.searchMessagesCmd(m.searchQuery)
}

func (m Model) openSelectedSearchResult() (Model, tea.Cmd) {
	if len(m.searchResults) == 0 || m.searchSelected >= len(m.searchResults) {
		return m, nil
	}
	chatID := m.searchResults[m.searchSelected].Message.ChatID
	if idx := chatIndexByID(m.chats, chatID); idx >= 0 {
		m.selected = idx
	}
	m.mode = ModeConversation
	m.currentChatID = chatID
	m.messages = nil
	m.msgOffset = 0
	m.convErr = nil
	m.loadingMsgs = true
	return m, tea.Batch(m.loadMessagesCmd(chatID), m.markReadCmd(chatID))
}

func chatIndexByID(chats []api.Chat, id string) int {
	for i, c := range chats {
		if c.ID == id {
			return i
		}
	}
	return -1
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
		m = m.cursorDown()
		return m, m.previewLoad()
	case "k", "up":
		m = m.cursorUp()
		return m, m.previewLoad()
	case "ctrl+d":
		m = m.halfPage(1)
		return m, m.previewLoad()
	case "ctrl+u":
		m = m.halfPage(-1)
		return m, m.previewLoad()
	case "l", "right", "tab":
		if m.mode == ModeList {
			m = m.cycleTab(1)
			return m, m.previewLoad()
		}
		return m, nil
	case "h", "left", "shift+tab":
		if m.mode == ModeList {
			m = m.cycleTab(-1)
			return m, m.previewLoad()
		}
		return m, nil
	case "G":
		m = m.jumpBottom()
		return m, m.previewLoad()
	case "g":
		if m.pendingG {
			m.pendingG = false
			m = m.jumpTop()
			return m, m.previewLoad()
		}
		m.pendingG = true
		return m, nil
	case "p":
		return m.togglePreview()
	case "i":
		if m.mode == ModeConversation {
			m.mode = ModeInsert
		}
		return m, nil
	case "a":
		return m.archiveSelected()
	case "enter":
		if m.mode == ModeList {
			return m.openSelected()
		}
		return m, nil
	}
	return m, nil
}
