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
type searchLoadedMsg struct {
	query   string
	results []api.MessageSearchResult
}
type errMsg struct {
	chatID      string // set for conversation-load errors; empty for chat-list errors
	searchQuery string // set for message-search errors
	err         error
}

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
			return errMsg{chatID: chatID, err: err}
		}
		return messagesLoadedMsg{chatID: chatID, messages: msgs}
	}
}

func (m Model) searchMessagesCmd(query string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		results, err := client.SearchMessages(ctx, query)
		if err != nil {
			return errMsg{searchQuery: query, err: err}
		}
		return searchLoadedMsg{query: query, results: results}
	}
}

type sendResultMsg struct {
	localID string
	err     error
}

type archiveResultMsg struct {
	chatID   string
	archived bool
	err      error
}

func (m Model) sendMessageCmd(chatID, localID, text string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := client.SendMessage(ctx, chatID, text)
		return sendResultMsg{localID: localID, err: err}
	}
}

func (m Model) archiveChatCmd(chatID string, archived bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := client.ArchiveChat(ctx, chatID, archived)
		return archiveResultMsg{chatID: chatID, archived: archived, err: err}
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
