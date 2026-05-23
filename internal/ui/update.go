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
		m.chats = msg.chats
		m.loadingChats = false
		return m, nil
	case messagesLoadedMsg:
		if msg.chatID == m.currentChatID {
			m.messages = msg.messages
			m.loadingMsgs = false
		}
		return m, nil
	case sendResultMsg:
		if msg.err != nil {
			if m.failedSends == nil {
				m.failedSends = map[string]bool{}
			}
			m.failedSends[msg.localID] = true
		}
		return m, nil
	case errMsg:
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
		return m.handleKey(msg.String())
	}
	return m, nil
}
