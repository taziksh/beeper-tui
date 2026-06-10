package ui

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) Init() tea.Cmd {
	return m.loadChatsCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chatsLoadedMsg:
		// Pin the user's selection across the re-sort by chat ID, so unread
		// chats can float to the top without the cursor jumping to a different
		// chat. An empty prior selection (initial load) resolves to index 0.
		var selectedID string
		if m.selected < len(m.chats) {
			selectedID = m.chats[m.selected].ID
		}
		chats := msg.chats
		sortChats(chats)
		m.chats = chats
		if selectedID != "" {
			m.selected = reselectByID(m.chats, selectedID)
		}
		m.loadingChats = false
		return m.clampWindow(), nil
	case messagesLoadedMsg:
		if msg.chatID == m.currentChatID {
			m.messages = msg.messages
			m.loadingMsgs = false
			// Land on the first unread message so new content is at the top of
			// the viewport; with nothing unread, fall back to the bottom.
			if u := firstUnreadIndex(m.messages); u >= 0 {
				m.msgOffset = u
				m = m.clampWindow()
			} else {
				m.msgOffset = m.maxMsgOffset()
			}
		}
		return m, nil
	case searchLoadedMsg:
		if m.mode == ModeSearch && msg.query == m.searchQuery {
			m.searchResults = msg.results
			m.searchSelected = 0
			m.searchOffset = 0
			m.searchLoading = false
			m.searchErr = nil
		}
		return m.clampWindow(), nil
	case sendResultMsg:
		if msg.err != nil {
			if m.failedSends == nil {
				m.failedSends = map[string]bool{}
			}
			m.failedSends[msg.localID] = true
		}
		return m, nil
	case archiveResultMsg:
		if msg.chatID != m.archivingChatID {
			return m, nil
		}
		m.archivingChatID = ""
		if msg.err != nil {
			m.archiveErr = msg.err
			return m, nil
		}
		m.archiveErr = nil
		return m.applyArchive(msg.chatID, msg.archived), nil
	case errMsg:
		if msg.searchQuery != "" {
			if m.mode == ModeSearch && msg.searchQuery == m.searchQuery {
				m.searchErr = msg.err
				m.searchLoading = false
			}
			return m, nil
		}
		if msg.chatID != "" {
			// Conversation-load error: scope it to the conversation body so the
			// list stays reachable via q. Ignore errors for a stale chat.
			if msg.chatID == m.currentChatID {
				m.convErr = msg.err
				m.loadingMsgs = false
			}
			return m, nil
		}
		m.err = msg.err
		m.loadingChats = false
		m.loadingMsgs = false
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.clampWindow(), nil
	case tea.KeyPressMsg:
		if m.mode == ModeInsert {
			return m.handleInsertKey(msg.String(), msg.Text)
		}
		if m.mode == ModeSearch {
			return m.handleSearchKey(msg.String(), msg.Text)
		}
		return m.handleKey(msg.String())
	}
	return m, nil
}
