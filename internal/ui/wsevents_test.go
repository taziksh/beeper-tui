package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/ws"
)

var wsT0 = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

func entryJSON(id, chatID, sender, text string, ts time.Time, fromMe, unread bool) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"id":%q,"chatID":%q,"senderName":%q,"text":%q,"timestamp":%q,"isSender":%t,"isUnread":%t}`,
		id, chatID, sender, text, ts.Format(time.RFC3339), fromMe, unread))
}

func upsertEvent(chatID string, entries ...json.RawMessage) ws.Event {
	return ws.Event{Type: ws.EventMessageUpserted, ChatID: chatID, Entries: entries}
}

func TestApplyWSEvent_ConnectionStates(t *testing.T) {
	m := Model{}

	m, cmd := m.applyWSEvent(ws.Event{Type: ws.EventConnecting})
	if m.conn != connConnecting || cmd != nil {
		t.Errorf("after connecting: conn = %v, cmd = %v; want connConnecting, nil", m.conn, cmd)
	}

	m, cmd = m.applyWSEvent(ws.Event{Type: ws.EventConnected})
	if m.conn != connConnected || !m.everConnected {
		t.Errorf("after first connect: conn = %v, everConnected = %t", m.conn, m.everConnected)
	}
	if cmd != nil {
		t.Error("first connect must not refetch; the initial load is already running")
	}

	m, _ = m.applyWSEvent(ws.Event{Type: ws.EventDisconnected, Err: errors.New("gone")})
	if m.conn != connDisconnected || m.connErr == nil {
		t.Errorf("after disconnect: conn = %v, connErr = %v", m.conn, m.connErr)
	}

	m, cmd = m.applyWSEvent(ws.Event{Type: ws.EventConnected})
	if cmd == nil {
		t.Error("reconnect must refetch to reconcile missed events")
	}
	if m.connErr != nil {
		t.Errorf("connErr = %v after reconnect, want nil", m.connErr)
	}
}

func TestApplyWSEvent_MessageUpserted_BumpsAndFloats(t *testing.T) {
	m := Model{
		chats: []api.Chat{
			{ID: "b", Title: "Bob", LastActive: wsT0},
			{ID: "a", Title: "Alice", LastActive: wsT0.Add(-time.Hour)},
		},
		selected: 0,
	}
	m, _ = m.applyWSEvent(upsertEvent("a", entryJSON("m1", "a", "Alice", "hello", wsT0.Add(time.Minute), false, true)))

	if m.chats[0].ID != "a" {
		t.Errorf("chats[0] = %q, want chat a floated to top", m.chats[0].ID)
	}
	if m.chats[0].Unread != 1 {
		t.Errorf("Unread = %d, want 1", m.chats[0].Unread)
	}
	if m.chats[0].Preview != "hello" {
		t.Errorf("Preview = %q, want %q", m.chats[0].Preview, "hello")
	}
	if m.chats[m.selected].ID != "b" {
		t.Errorf("selection moved to %q, want to stay on b", m.chats[m.selected].ID)
	}
}

func TestApplyWSEvent_MessageUpserted_FromMe_NoBump(t *testing.T) {
	m := Model{chats: []api.Chat{{ID: "a", LastActive: wsT0}}}
	m, _ = m.applyWSEvent(upsertEvent("a", entryJSON("m1", "a", "You", "sent elsewhere", wsT0.Add(time.Minute), true, false)))

	if m.chats[0].Unread != 0 {
		t.Errorf("Unread = %d, want 0 for own message", m.chats[0].Unread)
	}
	if m.chats[0].Preview != "sent elsewhere" {
		t.Errorf("Preview = %q, want updated", m.chats[0].Preview)
	}
}

func TestApplyWSEvent_MessageUpserted_Muted_NoFloat(t *testing.T) {
	m := Model{
		chats: []api.Chat{
			{ID: "b", LastActive: wsT0},
			{ID: "muted", Muted: true, LastActive: wsT0.Add(-time.Hour)},
		},
	}
	m, _ = m.applyWSEvent(upsertEvent("muted", entryJSON("m1", "muted", "Alice", "psst", wsT0.Add(time.Minute), false, true)))

	if m.chats[1].ID != "muted" {
		t.Error("muted chat must not float to the top")
	}
	if m.chats[1].Unread != 1 {
		t.Errorf("Unread = %d, want 1; muted still counts", m.chats[1].Unread)
	}
}

func TestApplyWSEvent_MessageUpserted_OpenAtBottom_ReadOnArrival(t *testing.T) {
	m := Model{
		chats:         []api.Chat{{ID: "a", LastActive: wsT0}},
		mode:          ModeConversation,
		currentChatID: "a",
		messages:      msgs(3),
		height:        7, // visibleRows 5: all messages fit, so offset 0 is the bottom
	}
	m, cmd := m.applyWSEvent(upsertEvent("a", entryJSON("m9", "a", "Alice", "new", wsT0.Add(time.Minute), false, true)))

	if m.chats[0].Unread != 0 {
		t.Errorf("Unread = %d, want 0 when read on arrival", m.chats[0].Unread)
	}
	if cmd == nil {
		t.Error("read on arrival must issue a mark-read command")
	}
	if len(m.messages) != 4 || m.messages[3].ID != "m9" {
		t.Fatalf("messages = %d entries, want the new message appended", len(m.messages))
	}
	if m.messages[3].IsUnread {
		t.Error("a message read on arrival must not render as unread")
	}
	if m.msgOffset != m.maxMsgOffset() {
		t.Errorf("msgOffset = %d, want pinned to bottom %d", m.msgOffset, m.maxMsgOffset())
	}
}

func TestApplyWSEvent_MessageUpserted_OpenScrolledUp_Bumps(t *testing.T) {
	m := Model{
		chats:         []api.Chat{{ID: "a", LastActive: wsT0}},
		mode:          ModeConversation,
		currentChatID: "a",
		messages:      msgs(20),
		msgOffset:     0, // scrolled to the top
		height:        7,
	}
	m, _ = m.applyWSEvent(upsertEvent("a", entryJSON("m9", "a", "Alice", "new", wsT0.Add(time.Minute), false, true)))

	if m.chats[0].Unread != 1 {
		t.Errorf("Unread = %d, want 1 when scrolled away", m.chats[0].Unread)
	}
	if m.msgOffset != 0 {
		t.Errorf("msgOffset = %d, want 0; arrival must not yank the reader", m.msgOffset)
	}
	if len(m.messages) != 21 {
		t.Errorf("messages = %d, want appended", len(m.messages))
	}
}

func TestApplyWSEvent_MessageUpserted_OptimisticEcho_ReplacesPlaceholder(t *testing.T) {
	m := Model{
		chats:         []api.Chat{{ID: "a", LastActive: wsT0}},
		mode:          ModeConversation,
		currentChatID: "a",
		messages: []api.Message{
			{ID: "local:1", ChatID: "a", Text: "hi there", IsFromMe: true},
		},
		failedSends: map[string]bool{"local:1": true},
		height:      7,
	}
	m, _ = m.applyWSEvent(upsertEvent("a", entryJSON("srv9", "a", "You", "hi there", wsT0.Add(time.Minute), true, false)))

	if len(m.messages) != 1 {
		t.Fatalf("messages = %d, want the echo to replace the placeholder, not append", len(m.messages))
	}
	if m.messages[0].ID != "srv9" {
		t.Errorf("ID = %q, want server ID srv9", m.messages[0].ID)
	}
	if m.failedSends["local:1"] {
		t.Error("server echo must clear the failed-send flag")
	}
}

func TestApplyWSEvent_MessageUpserted_DuplicateID_ReconcilesInPlace(t *testing.T) {
	m := Model{
		chats:         []api.Chat{{ID: "a", LastActive: wsT0}},
		mode:          ModeConversation,
		currentChatID: "a",
		messages:      []api.Message{{ID: "m1", ChatID: "a", Text: "before edit"}},
		height:        7,
	}
	m, _ = m.applyWSEvent(upsertEvent("a", entryJSON("m1", "a", "Alice", "after edit", wsT0.Add(time.Minute), false, false)))

	if len(m.messages) != 1 {
		t.Fatalf("messages = %d, want edit applied in place", len(m.messages))
	}
	if m.messages[0].Text != "after edit" {
		t.Errorf("Text = %q, want %q", m.messages[0].Text, "after edit")
	}
}

func TestApplyWSEvent_MessageUpserted_UnknownChat_RefetchesList(t *testing.T) {
	m := Model{chats: []api.Chat{{ID: "a"}}}
	m, cmd := m.applyWSEvent(upsertEvent("new-chat", entryJSON("m1", "new-chat", "Carol", "hi", wsT0, false, true)))

	if cmd == nil {
		t.Error("a message for an unknown chat must refetch the chat list")
	}
	if len(m.chats) != 1 {
		t.Errorf("chats = %d, want unchanged until the refetch lands", len(m.chats))
	}
}

func TestApplyWSEvent_MessageDeleted_RemovesFromOpenConversation(t *testing.T) {
	m := Model{
		chats:         []api.Chat{{ID: "a"}},
		mode:          ModeConversation,
		currentChatID: "a",
		messages: []api.Message{
			{ID: "m1", ChatID: "a"}, {ID: "m2", ChatID: "a"}, {ID: "m3", ChatID: "a"},
		},
		previewCache: map[string][]api.Message{"a": {{ID: "m2"}}},
		height:       7,
	}
	m, _ = m.applyWSEvent(ws.Event{Type: ws.EventMessageDeleted, ChatID: "a", IDs: []string{"m2"}})

	if len(m.messages) != 2 || m.messages[0].ID != "m1" || m.messages[1].ID != "m3" {
		t.Errorf("messages = %+v, want m2 removed", m.messages)
	}
	if _, ok := m.previewCache["a"]; ok {
		t.Error("preview cache must be dropped so the deleted message disappears there too")
	}
}

func TestApplyWSEvent_ChatUpserted_TriggersRefetch(t *testing.T) {
	m := Model{chats: []api.Chat{{ID: "a"}}}
	_, cmd := m.applyWSEvent(ws.Event{Type: ws.EventChatUpserted, ChatID: "a", IDs: []string{"a"}})
	if cmd == nil {
		t.Error("chat.upserted carries no entries, so it must refetch the chat")
	}
}

func TestApplyChatRefreshed_MergesPreservingPreview(t *testing.T) {
	m := Model{
		chats: []api.Chat{
			{ID: "b", LastActive: wsT0},
			{ID: "a", Unread: 3, Preview: "old preview", LastActive: wsT0.Add(-time.Hour)},
		},
	}
	m = m.applyChatRefreshed(api.Chat{ID: "a", Unread: 0, LastActive: wsT0.Add(time.Minute)})

	if m.chats[0].ID != "a" {
		t.Errorf("chats[0] = %q, want refreshed chat re-sorted to top", m.chats[0].ID)
	}
	if m.chats[0].Unread != 0 {
		t.Errorf("Unread = %d, want 0; read-elsewhere must clear the badge", m.chats[0].Unread)
	}
	if m.chats[0].Preview != "old preview" {
		t.Errorf("Preview = %q, want preserved across the merge", m.chats[0].Preview)
	}
}

func TestApplyChatRefreshed_NewChatAppends(t *testing.T) {
	m := Model{chats: []api.Chat{{ID: "a"}}}
	m = m.applyChatRefreshed(api.Chat{ID: "new", Title: "Carol"})
	if len(m.chats) != 2 {
		t.Errorf("chats = %d, want the new chat added", len(m.chats))
	}
}

func TestApplyWSEvent_ChatDeleted_OpenChatFallsBack(t *testing.T) {
	m := Model{
		chats:         []api.Chat{{ID: "a"}, {ID: "b"}},
		mode:          ModeConversation,
		currentChatID: "b",
		messages:      msgs(2),
		selected:      1,
		previewCache:  map[string][]api.Message{"b": {{ID: "m1"}}},
	}
	m, _ = m.applyWSEvent(ws.Event{Type: ws.EventChatDeleted, ChatID: "b", IDs: []string{"b"}})

	if m.mode != ModeList {
		t.Errorf("mode = %v, want fallback to the list", m.mode)
	}
	if len(m.chats) != 1 || m.chats[0].ID != "a" {
		t.Errorf("chats = %+v, want only a", m.chats)
	}
	if m.currentChatID != "" || len(m.messages) != 0 {
		t.Error("conversation state must be cleared for a deleted chat")
	}
	if m.selected != 0 {
		t.Errorf("selected = %d, want reselected onto a remaining chat", m.selected)
	}
}

func TestApplyWSEvent_MessageUpserted_UpdatesPreviewCache(t *testing.T) {
	m := Model{
		chats:        []api.Chat{{ID: "a", LastActive: wsT0}},
		previewCache: map[string][]api.Message{"a": {{ID: "m1", Text: "old"}}},
	}
	m, _ = m.applyWSEvent(upsertEvent("a", entryJSON("m2", "a", "Alice", "fresh", wsT0.Add(time.Minute), false, true)))

	cached := m.previewCache["a"]
	if len(cached) != 2 || cached[1].Text != "fresh" {
		t.Errorf("previewCache = %+v, want the live message appended", cached)
	}
}

func TestStatusBar_ShowsConnectionState(t *testing.T) {
	m := Model{conn: connDisconnected}
	if got := m.statusBar(); !strings.Contains(got, "offline") {
		t.Errorf("statusBar() = %q, want an offline indicator", got)
	}
	m.conn = connConnected
	if got := m.statusBar(); strings.Contains(got, "offline") || strings.Contains(got, "connecting") {
		t.Errorf("statusBar() = %q, want quiet when connected", got)
	}
}
