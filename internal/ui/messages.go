package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
)

type chatsLoadedMsg struct{ chats []api.Chat }
type messagesLoadedMsg struct {
	chatID   string
	messages []api.Message
}
type errMsg struct{ err error }

func (m Model) loadChatsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		chats, err := client.ListChats(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return chatsLoadedMsg{chats: chats}
	}
}

func (m Model) loadMessagesCmd(chatID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		msgs, err := client.ListMessages(ctx, chatID)
		if err != nil {
			return errMsg{err: err}
		}
		return messagesLoadedMsg{chatID: chatID, messages: msgs}
	}
}

// markReadCmd marks a chat read, best-effort (read state isn't worth surfacing
// an error for).
func (m Model) markReadCmd(chatID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.MarkRead(ctx, chatID)
		return nil
	}
}
