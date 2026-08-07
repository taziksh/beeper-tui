package ui

import (
	"context"
	"errors"
	"fmt"
	"syscall"
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
	text    string
	atts    []string
	err     error
}

type archiveResultMsg struct {
	chatID   string
	archived bool
	err      error
}

// sendErrReason condenses a send error to a short cause for the failed-send
// flag in the conversation view.
func sendErrReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded):
		return "timed out"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "beeper not running"
	}
	if s := api.ErrorStatus(err); s != 0 {
		return fmt.Sprintf("HTTP %d", s)
	}
	root := err
	for u := errors.Unwrap(root); u != nil; u = errors.Unwrap(u) {
		root = u
	}
	return truncate(root.Error(), 40)
}

// retryFailedSend re-sends the failed message under the cursor from its
// recorded draft.
func (m Model) retryFailedSend() (Model, tea.Cmd) {
	msg := m.cursorMessage()
	if msg == nil {
		return m, nil
	}
	f, ok := m.failedSends[msg.ID]
	if !ok {
		return m, nil
	}
	delete(m.failedSends, msg.ID)
	if len(f.atts) > 0 {
		return m, m.sendAttachmentsCmd(msg.ChatID, msg.ID, f.text, f.atts)
	}
	return m, m.sendMessageCmd(msg.ChatID, msg.ID, f.text)
}

func (m Model) sendMessageCmd(chatID, localID, text string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := client.SendMessage(ctx, chatID, text)
		return sendResultMsg{localID: localID, text: text, err: err}
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
