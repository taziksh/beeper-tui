package ui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
)

// pollInterval is the REST refresh cadence backstopping bridges that emit no
// WebSocket events (iMessage, as of Beeper Desktop 4.2.x). Events still
// deliver instantly where emission works; polling only bounds the staleness
// everywhere else.
const pollInterval = 30 * time.Second

type pollTickMsg struct{}

// messagesRefreshedMsg is the background counterpart of messagesLoadedMsg; it
// must not reposition the reader's scroll.
type messagesRefreshedMsg struct {
	chatID   string
	messages []api.Message
}

func pollTick() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg { return pollTickMsg{} })
}

// applyPollTick fires the background refreshes and schedules the next tick.
func (m Model) applyPollTick() (Model, tea.Cmd) {
	cmds := []tea.Cmd{pollTick(), m.refreshChatsCmd(), m.loadContactsCmd()}
	if m.currentChatID != "" && (m.mode == ModeConversation || m.mode == ModeInsert) {
		cmds = append(cmds, m.refreshMessagesCmd(m.currentChatID))
	}
	return m, tea.Batch(cmds...)
}

// refreshChatsCmd refetches the chat list silently: a failed background poll
// changes nothing instead of replacing the UI with an error screen.
func (m Model) refreshChatsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pollInterval)
		defer cancel()
		chats, err := client.ListChats(ctx)
		if err != nil {
			return nil
		}
		return chatsLoadedMsg{chats: chats}
	}
}

func (m Model) refreshMessagesCmd(chatID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pollInterval)
		defer cancel()
		msgs, err := client.ListMessages(ctx, chatID)
		if err != nil {
			return nil
		}
		return messagesRefreshedMsg{chatID: chatID, messages: msgs}
	}
}

// applyMessagesRefreshed swaps in the refetched conversation, keeping the
// reader's scroll position except at the bottom, which sticks to the new
// bottom. Optimistic sends the server has not echoed yet are re-appended so
// a poll cannot make a just-sent message vanish. An optimistic send the
// server HAS echoed is dropped along with its failure flag: the send call
// can return an error after the bridge already dispatched the message
// (iMessage attachments do this), and the server's copy is the truth.
func (m Model) applyMessagesRefreshed(msg messagesRefreshedMsg) Model {
	if msg.chatID != m.currentChatID {
		return m
	}
	atBottom := m.msgOffset >= m.maxMsgOffset()
	merged := msg.messages
	for _, old := range m.messages {
		if !strings.HasPrefix(old.ID, "local:") {
			continue
		}
		if hasEcho(merged, old) {
			delete(m.failedSends, old.ID)
			continue
		}
		merged = append(merged, old)
	}
	m.messages = merged
	if atBottom {
		m.msgSelected = len(m.messages) - 1
		m.msgOffset = m.maxMsgOffset()
	}
	return m.clampWindow()
}

// hasEcho reports whether the server already carries an optimistic local
// send. Matching is by from-me + same text + same has-attachments shape, and
// the server copy must not predate the local send by more than a clock-skew
// allowance, so an older identical message doesn't swallow a genuinely
// unsent one.
func hasEcho(msgs []api.Message, local api.Message) bool {
	cutoff := local.Timestamp.Add(-2 * time.Minute)
	for _, msg := range msgs {
		if !msg.IsFromMe || msg.Text != local.Text {
			continue
		}
		if (len(msg.Attachments) > 0) != (len(local.Attachments) > 0) {
			continue
		}
		if msg.Timestamp.IsZero() || msg.Timestamp.After(cutoff) {
			return true
		}
	}
	return false
}
