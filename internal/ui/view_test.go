package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestRender_LoadingChats(t *testing.T) {
	m := Model{mode: ModeList, loadingChats: true, width: 80, height: 24}
	if !strings.Contains(m.render(), "Loading") {
		t.Errorf("loading view missing 'Loading': %q", m.render())
	}
}

func TestFormatMessageTime_UsesLocalClockTime(t *testing.T) {
	loc := time.FixedZone("PDT", -7*60*60)
	ts := time.Date(2026, 6, 1, 19, 30, 0, 0, time.UTC)
	now := time.Date(2026, 6, 1, 13, 0, 0, 0, loc)
	if got := formatMessageTime(ts, now); got != "12:30" {
		t.Errorf("formatMessageTime = %q, want local time 12:30", got)
	}
}

func TestFormatMessageTime_IncludesDateForOlderMessages(t *testing.T) {
	loc := time.FixedZone("PDT", -7*60*60)
	ts := time.Date(2026, 5, 31, 19, 30, 0, 0, loc)
	now := time.Date(2026, 6, 1, 13, 0, 0, 0, loc)
	if got := formatMessageTime(ts, now); got != "May 31 19:30" {
		t.Errorf("formatMessageTime = %q, want May 31 19:30", got)
	}
}

func TestFormatMessageTime_IncludesYearForOlderYears(t *testing.T) {
	loc := time.FixedZone("PDT", -7*60*60)
	ts := time.Date(2025, 12, 31, 19, 30, 0, 0, loc)
	now := time.Date(2026, 6, 1, 13, 0, 0, 0, loc)
	if got := formatMessageTime(ts, now); got != "2025-12-31 19:30" {
		t.Errorf("formatMessageTime = %q, want 2025-12-31 19:30", got)
	}
}

func TestFormatMessageTime_ZeroTimestamp(t *testing.T) {
	if got := formatMessageTime(time.Time{}, time.Now()); got != "--:--" {
		t.Errorf("formatMessageTime = %q, want --:--", got)
	}
}

func TestRender_ListShowsTitles(t *testing.T) {
	m := Model{
		mode: ModeList, width: 80, height: 24,
		chats: []api.Chat{
			{ID: "a", Network: "Signal", Title: "Alice", Unread: 0},
			{ID: "b", Network: "WhatsApp", Title: "Dev Team", Unread: 5},
		},
		selected: 0,
	}
	out := m.render()
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Dev Team") {
		t.Errorf("list missing titles: %q", out)
	}
	if !strings.Contains(out, "5") {
		t.Errorf("list missing unread count: %q", out)
	}
}

func TestRender_ConversationShowsMessages(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages: []api.Message{
			{ID: "m1", SenderName: "Alice", Text: "hey there", IsFromMe: false},
			{ID: "m2", SenderName: "Me", Text: "hi back", IsFromMe: true},
		},
	}
	out := m.render()
	if !strings.Contains(out, "hey there") || !strings.Contains(out, "hi back") {
		t.Errorf("conversation missing message text: %q", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("conversation missing chat title: %q", out)
	}
}

func TestRender_InsertShowsComposeLine(t *testing.T) {
	m := Model{
		mode: ModeInsert, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages:      []api.Message{{ID: "m1", SenderName: "Alice", Text: "hi"}},
		input:         "see you at 7",
	}
	out := m.render()
	if !strings.Contains(out, "> see you at 7") {
		t.Errorf("INSERT view missing compose line with draft: %q", out)
	}
	if !strings.Contains(out, "INSERT") {
		t.Errorf("INSERT view missing INSERT in status bar: %q", out)
	}
}

func TestRender_NormalConversationHasNoComposeLine(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages:      []api.Message{{ID: "m1", SenderName: "Alice", Text: "hi"}},
		input:         "leftover",
	}
	out := m.render()
	if strings.Contains(out, "> leftover") {
		t.Errorf("NORMAL conversation must not show a compose line: %q", out)
	}
	if !strings.Contains(out, "NORMAL") {
		t.Errorf("NORMAL conversation missing NORMAL in status bar: %q", out)
	}
}

func TestRender_FailedSendMarker(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages:      []api.Message{{ID: "local:1", SenderName: "You", Text: "nope", IsFromMe: true}},
		failedSends:   map[string]bool{"local:1": true},
	}
	out := m.render()
	if !strings.Contains(out, "! send failed") {
		t.Errorf("failed optimistic message missing marker: %q", out)
	}
}

func TestRender_ConversationLoadError_ShownInBody(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		convErr:       errors.New("ListMessages failed: read error"),
	}
	out := m.render()
	if !strings.Contains(out, "read error") {
		t.Errorf("conversation error should appear in the body: %q", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("conversation header (chat title) should still show: %q", out)
	}
	if !strings.Contains(out, "q chats") {
		t.Errorf("status bar with q hint should still show so the user can get out: %q", out)
	}
}

func TestRender_ConversationLoadError_WordWrapped(t *testing.T) {
	long := errors.New(strings.TrimSpace(strings.Repeat("connection reset by peer ", 12)))
	m := Model{
		mode: ModeConversation, width: 70, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		convErr:       long,
	}
	out := m.render()
	for _, line := range strings.Split(out, "\n") {
		if len([]rune(line)) > 70 {
			t.Errorf("line exceeds width 70 (error not word-wrapped): %q", line)
		}
	}
	if !strings.Contains(out, "connection") || !strings.Contains(out, "peer") {
		t.Errorf("word-wrapped error lost content: %q", out)
	}
}

func TestRender_UnreadChatHasGlyph(t *testing.T) {
	m := Model{
		mode: ModeList, width: 80, height: 24,
		chats: []api.Chat{
			{ID: "a", Network: "Signal", Title: "Alice", Unread: 0},
			{ID: "b", Network: "WhatsApp", Title: "Dev Team", Unread: 5},
		},
		selected: 0,
	}
	out := m.render()
	if !strings.Contains(out, "●") {
		t.Errorf("unread chat row missing ● glyph: %q", out)
	}
}

func TestRender_UnreadMessageHasMarker(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages: []api.Message{
			{ID: "m1", SenderName: "Alice", Text: "seen this", IsUnread: false},
			{ID: "m2", SenderName: "Alice", Text: "brand new", IsUnread: true},
		},
	}
	out := m.render()
	if !strings.Contains(out, "▎") {
		t.Errorf("unread message row missing ▎ marker: %q", out)
	}
}

func TestRender_LowPrioritySectionDivider(t *testing.T) {
	m := Model{
		mode: ModeList, width: 40, height: 24,
		chats: []api.Chat{
			{ID: "a", Network: "Signal", Title: "Alice", Unread: 1},
			{ID: "b", Network: "WhatsApp", Title: "Muted Group", Muted: true, Unread: 9},
		},
		selected: 0,
	}
	out := m.render()
	if !strings.Contains(out, "low priority") {
		t.Fatalf("expected low-priority divider in output: %q", out)
	}
	// Divider sits after the normal chat and before the muted one.
	ai := strings.Index(out, "Alice")
	di := strings.Index(out, "low priority")
	mi := strings.Index(out, "Muted Group")
	if !(ai < di && di < mi) {
		t.Errorf("order wrong: Alice@%d divider@%d Muted@%d (want Alice<divider<Muted)", ai, di, mi)
	}
}

func TestRender_ConversationLoading(t *testing.T) {
	m := Model{
		mode: ModeConversation, loadingMsgs: true, width: 80, height: 24,
		chats: []api.Chat{{ID: "a", Title: "Alice"}}, currentChatID: "a",
	}
	if !strings.Contains(m.render(), "Loading") {
		t.Errorf("expected loading text: %q", m.render())
	}
}
