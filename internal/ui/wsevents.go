package ui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/ws"
)

type wsEventMsg struct{ event ws.Event }

// chatRefreshedMsg carries one chat refetched after a chat.upserted event,
// which arrives without entries on the wire.
type chatRefreshedMsg struct{ chat api.Chat }

// waitForWSEvent blocks on the events channel and wraps one event as a
// tea.Msg. The reducer re-issues it after each event, feeding the stream
// into Update one message at a time.
func (m Model) waitForWSEvent() tea.Cmd {
	if m.events == nil {
		return nil
	}
	ch := m.events.Events()
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return nil
		}
		return wsEventMsg{event: e}
	}
}

// retryConnection forces an immediate reconnect attempt and, when the chat
// list failed to load, retries that too.
func (m Model) retryConnection() (Model, tea.Cmd) {
	events := m.events
	retry := func() tea.Msg {
		if events != nil {
			events.Retry()
		}
		return nil
	}
	if m.err != nil {
		m.err = nil
		m.loadingChats = true
		return m, tea.Batch(retry, m.loadChatsCmd())
	}
	return m, retry
}

func (m Model) applyWSEvent(e ws.Event) (Model, tea.Cmd) {
	switch e.Type {
	case ws.EventConnecting:
		m.conn = connConnecting
		return m, nil
	case ws.EventConnected:
		m.conn = connConnected
		m.connErr = nil
		if !m.everConnected {
			m.everConnected = true
			return m, nil
		}
		return m.reconcileAfterReconnect()
	case ws.EventDisconnected:
		m.conn = connDisconnected
		m.connErr = e.Err
		return m, nil
	case ws.EventMessageUpserted:
		return m.applyMessageUpserted(e)
	case ws.EventMessageDeleted:
		return m.applyMessageDeleted(e), nil
	case ws.EventChatUpserted:
		return m, m.refreshChatCmd(eventChatID(e))
	case ws.EventChatDeleted:
		return m.applyChatDeleted(e), nil
	}
	return m, nil
}

// reconcileAfterReconnect refetches REST state to cover events missed while
// disconnected: the chat list, the open conversation, and preview caches.
func (m Model) reconcileAfterReconnect() (Model, tea.Cmd) {
	m.previewCache = nil
	m.previewErr = nil
	cmds := []tea.Cmd{m.loadChatsCmd()}
	if m.currentChatID != "" {
		cmds = append(cmds, m.loadMessagesCmd(m.currentChatID))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) applyMessageUpserted(e ws.Event) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, raw := range e.Entries {
		msg, err := api.MessageFromJSON(raw)
		if err != nil {
			continue
		}
		if msg.ChatID == "" {
			msg.ChatID = e.ChatID
		}
		// A reaction event is not a timeline message: no row, no unread bump,
		// no preview. It does mean the reacted message changed, so refetch the
		// open conversation to update reaction suffixes live.
		if msg.IsReaction {
			if msg.ChatID == m.currentChatID && m.mode != ModeList {
				cmds = append(cmds, m.refreshMessagesCmd(msg.ChatID))
			}
			continue
		}
		var cmd tea.Cmd
		m, cmd = m.applyIncomingMessage(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

// applyIncomingMessage folds one live message into the chat list, the open
// conversation, and the preview cache. A message from the user never bumps
// the unread count, one arriving while its chat is open and scrolled to the
// bottom is read on arrival, and anything else bumps the badge. Muted and
// low-priority chats keep their LastActive so they never float to the top.
func (m Model) applyIncomingMessage(msg api.Message) (Model, tea.Cmd) {
	idx := chatIndexByID(m.chats, msg.ChatID)
	if idx < 0 {
		// A chat this client has never seen; refetch the list to pick it up.
		return m, m.loadChatsCmd()
	}

	open := m.currentChatID == msg.ChatID && (m.mode == ModeConversation || m.mode == ModeInsert)
	atBottom := m.msgOffset >= m.maxMsgOffset()
	readOnArrival := open && atBottom

	var cmd tea.Cmd
	m.chats[idx].Preview = msg.Text
	switch {
	case msg.IsFromMe:
	case readOnArrival:
		cmd = m.markReadCmd(msg.ChatID)
	default:
		m.chats[idx].Unread++
	}
	if !m.chats[idx].Muted && !m.chats[idx].LowPriority && msg.Timestamp.After(m.chats[idx].LastActive) {
		m.chats[idx].LastActive = msg.Timestamp
	}

	if readOnArrival {
		msg.IsUnread = false
	}
	if open {
		m = m.upsertConversationMessage(msg, atBottom)
	}
	if cached, ok := m.previewCache[msg.ChatID]; ok {
		m.previewCache[msg.ChatID] = upsertMessage(cached, msg)
	}

	return m.resortKeepingSelection(), cmd
}

// upsertConversationMessage adds a live message to the open conversation. A
// from-me message first tries to claim an optimistic send placeholder, and
// any message replaces an existing one with the same ID instead of
// double-rendering.
func (m Model) upsertConversationMessage(msg api.Message, atBottom bool) Model {
	if msg.IsFromMe {
		for i, existing := range m.messages {
			if strings.HasPrefix(existing.ID, "local:") && existing.Text == msg.Text {
				delete(m.failedSends, existing.ID)
				m.messages[i] = msg
				return m
			}
		}
	}
	m.messages = upsertMessage(m.messages, msg)
	if atBottom {
		// Follow the conversation: keep the cursor on the newest message.
		m.msgSelected = len(m.messages) - 1
		m.msgOffset = m.maxMsgOffset()
	}
	return m
}

func upsertMessage(msgs []api.Message, msg api.Message) []api.Message {
	for i, existing := range msgs {
		if existing.ID == msg.ID {
			msgs[i] = msg
			return msgs
		}
	}
	return append(msgs, msg)
}

func (m Model) applyMessageDeleted(e ws.Event) Model {
	chatID := eventChatID(e)
	deleted := make(map[string]bool, len(e.IDs))
	for _, id := range e.IDs {
		deleted[id] = true
	}
	if m.currentChatID == chatID {
		kept := m.messages[:0]
		for _, msg := range m.messages {
			if !deleted[msg.ID] {
				kept = append(kept, msg)
			}
		}
		m.messages = kept
		m = m.clampWindow()
	}
	// The cached preview may show a deleted message; drop it so the next view
	// refetches.
	delete(m.previewCache, chatID)
	return m
}

func (m Model) applyChatDeleted(e ws.Event) Model {
	deleted := make(map[string]bool, len(e.IDs)+1)
	for _, id := range e.IDs {
		deleted[id] = true
	}
	if e.ChatID != "" {
		deleted[e.ChatID] = true
	}
	var selectedID string
	if m.selected < len(m.chats) {
		selectedID = m.chats[m.selected].ID
	}
	kept := m.chats[:0]
	for _, c := range m.chats {
		if !deleted[c.ID] {
			kept = append(kept, c)
		}
	}
	m.chats = kept
	for id := range deleted {
		delete(m.previewCache, id)
		delete(m.previewErr, id)
	}
	if deleted[m.currentChatID] && (m.mode == ModeConversation || m.mode == ModeInsert) {
		m = m.backToList()
		m.currentChatID = ""
		m.messages = nil
		m.msgOffset = 0
		m.loadingMsgs = false
		m.input = ""
	}
	m.selected = reselectByID(m.chats, selectedID)
	return m.selectFirstVisibleChat()
}

// refreshChatCmd refetches one chat after a chat.upserted event, best-effort.
func (m Model) refreshChatCmd(chatID string) tea.Cmd {
	if chatID == "" {
		return nil
	}
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		chat, err := client.GetChat(ctx, chatID)
		if err != nil {
			return nil
		}
		return chatRefreshedMsg{chat: chat}
	}
}

// applyChatRefreshed merges a refetched chat into the list. The single-chat
// endpoint returns no preview text, so a present preview survives the merge.
func (m Model) applyChatRefreshed(chat api.Chat) Model {
	if idx := chatIndexByID(m.chats, chat.ID); idx >= 0 {
		if chat.Preview == "" {
			chat.Preview = m.chats[idx].Preview
		}
		m.chats[idx] = chat
	} else {
		m.chats = append(m.chats, chat)
	}
	return m.resortKeepingSelection()
}

// resortKeepingSelection re-sorts the chat list while the cursor stays on the
// same chat.
func (m Model) resortKeepingSelection() Model {
	var selectedID string
	if m.selected < len(m.chats) {
		selectedID = m.chats[m.selected].ID
	}
	sortChats(m.chats)
	if selectedID != "" {
		m.selected = reselectByID(m.chats, selectedID)
	}
	return m.clampWindow()
}

// eventChatID prefers the envelope chatID, falling back to the first ID for
// chat-scoped events.
func eventChatID(e ws.Event) string {
	if e.ChatID != "" {
		return e.ChatID
	}
	if len(e.IDs) > 0 {
		return e.IDs[0]
	}
	return ""
}

// connStatus is the status-bar segment for the live-events connection. It is
// empty when connected or when live updates are disabled.
func (m Model) connStatus() string {
	switch m.conn {
	case connConnecting:
		if m.everConnected {
			return "reconnecting… · "
		}
		return "connecting… · "
	case connDisconnected:
		return "offline, retrying (r: retry now) · "
	default:
		return ""
	}
}
