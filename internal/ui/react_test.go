package ui

import (
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func reactModel() Model {
	return Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", AccountID: "acc", Title: "Alice"}},
		selfUsers:     map[string]string{"acc": "me"},
		messages: []api.Message{
			{ID: "m1", SenderName: "Alice", Text: "hey"},
			{ID: "m2", SenderName: "Alice", Text: "lunch?"},
		},
		msgSelected: 1,
	}
}

func TestReactKey_OpensPicker(t *testing.T) {
	m := reactModel()
	m, _ = m.handleKey("r")
	if m.mode != ModeReact {
		t.Fatalf("mode = %v, want ModeReact", m.mode)
	}
	out := m.render()
	if !strings.Contains(out, "react: 1 👍") {
		t.Errorf("picker line missing presets: %q", out)
	}
	if !strings.Contains(out, "REACT") {
		t.Errorf("status bar missing REACT: %q", out)
	}
}

func TestReactDigit_AddsOptimisticallyAndFiresCmd(t *testing.T) {
	m := reactModel()
	m, _ = m.handleKey("r")
	m, cmd := m.handleReactKey("1", "1")
	if cmd == nil {
		t.Fatal("digit pick must fire a react command")
	}
	if m.mode != ModeConversation {
		t.Errorf("mode = %v, want back to ModeConversation", m.mode)
	}
	rs := m.messages[1].Reactions
	if len(rs) != 1 || rs[0].Key != "👍" || rs[0].ParticipantID != "me" {
		t.Errorf("Reactions = %+v, want own 👍", rs)
	}
}

func TestReactDigit_TogglesOwnReactionOff(t *testing.T) {
	m := reactModel()
	m.messages[1].Reactions = []api.Reaction{
		{Key: "👍", Emoji: true, ParticipantID: "me"},
		{Key: "👍", Emoji: true, ParticipantID: "other"},
	}
	m, _ = m.handleKey("r")
	// Slot 1 is the existing 👍.
	m, cmd := m.handleReactKey("1", "1")
	if cmd == nil {
		t.Fatal("toggle must fire a react command")
	}
	rs := m.messages[1].Reactions
	if len(rs) != 1 || rs[0].ParticipantID != "other" {
		t.Errorf("Reactions = %+v, want only the other participant's kept", rs)
	}
}

func TestReactTyped_SendsOnEnter(t *testing.T) {
	m := reactModel()
	m, _ = m.handleKey("r")
	m, _ = m.handleReactKey("🔥", "🔥")
	m, cmd := m.handleReactKey("enter", "")
	if cmd == nil {
		t.Fatal("enter with typed emoji must fire a react command")
	}
	rs := m.messages[1].Reactions
	if len(rs) != 1 || rs[0].Key != "🔥" || !rs[0].Emoji {
		t.Errorf("Reactions = %+v, want typed 🔥", rs)
	}
}

func TestReactEsc_CancelsWithoutChange(t *testing.T) {
	m := reactModel()
	m, _ = m.handleKey("r")
	m, cmd := m.handleReactKey("esc", "")
	if cmd != nil || m.mode != ModeConversation || len(m.messages[1].Reactions) != 0 {
		t.Errorf("esc must cancel cleanly: mode=%v reactions=%+v", m.mode, m.messages[1].Reactions)
	}
}

func TestReactSlots_ExistingReactionsFirstThenAllowed(t *testing.T) {
	m := reactModel()
	m.chats[0].AllowedReactions = []string{"💯", "👍"}
	m.messages[1].Reactions = []api.Reaction{{Key: "😂", Emoji: true, ParticipantID: "other"}}
	slots := m.reactSlots()
	want := []string{"😂", "💯", "👍"}
	if len(slots) != len(want) {
		t.Fatalf("slots = %v, want %v", slots, want)
	}
	for i := range want {
		if slots[i] != want[i] {
			t.Fatalf("slots = %v, want %v", slots, want)
		}
	}
}

func TestFormatReactions_MarksOwn(t *testing.T) {
	got := formatReactions([]api.Reaction{
		{Key: "👍", Emoji: true, ParticipantID: "me"},
		{Key: "👍", Emoji: true, ParticipantID: "other"},
		{Key: "❤️", Emoji: true, ParticipantID: "other"},
	}, "me")
	if got != "👍 2*  ❤️ 1" {
		t.Errorf("formatReactions = %q, want \"👍 2*  ❤️ 1\"", got)
	}
}

func TestRender_ConversationCursor(t *testing.T) {
	m := reactModel()
	out := m.render()
	if !strings.Contains(out, ">") || !strings.Contains(out, "lunch?") {
		t.Errorf("conversation missing cursor or message: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, ">") && !strings.Contains(line, "lunch?") {
			t.Errorf("cursor on wrong line: %q", line)
		}
	}
}

func TestEmojiCandidates_ExactSlugFirst(t *testing.T) {
	cands := emojiCandidates("fire", 5)
	if len(cands) == 0 || cands[0] != "🔥" {
		t.Errorf("emojiCandidates(fire) = %v, want 🔥 first", cands)
	}
}

func TestEmojiCandidates_SpacesMatchKebabSlugs(t *testing.T) {
	cands := emojiCandidates("thumbs up", 5)
	if len(cands) == 0 || cands[0] != "👍" {
		t.Errorf("emojiCandidates(thumbs up) = %v, want 👍 first", cands)
	}
}

func TestEmojiCandidates_EmptyAndEmojiQueries(t *testing.T) {
	if got := emojiCandidates("", 5); got != nil {
		t.Errorf("emojiCandidates(empty) = %v, want nil", got)
	}
	if got := emojiCandidates("🔥", 5); got != nil {
		t.Errorf("emojiCandidates(🔥) = %v, want nil", got)
	}
}

func TestReactTyped_FuzzyMatchSendsEmojiOnEnter(t *testing.T) {
	m := reactModel()
	m, _ = m.handleKey("r")
	for _, ch := range []string{"f", "i", "r", "e"} {
		m, _ = m.handleReactKey(ch, ch)
	}
	m, cmd := m.handleReactKey("enter", "")
	if cmd == nil {
		t.Fatal("enter with fuzzy match must fire a react command")
	}
	rs := m.messages[1].Reactions
	if len(rs) != 1 || rs[0].Key != "🔥" {
		t.Errorf("Reactions = %+v, want fuzzy-matched 🔥", rs)
	}
}

func TestReactTab_CyclesCandidates(t *testing.T) {
	m := reactModel()
	m, _ = m.handleKey("r")
	for _, ch := range []string{"f", "i", "r", "e"} {
		m, _ = m.handleReactKey(ch, ch)
	}
	m, _ = m.handleReactKey("tab", "")
	if m.reactCandIdx != 1 {
		t.Errorf("reactCandIdx = %d, want 1 after tab", m.reactCandIdx)
	}
	m, _ = m.handleReactKey("shift+tab", "")
	if m.reactCandIdx != 0 {
		t.Errorf("reactCandIdx = %d, want 0 after shift+tab", m.reactCandIdx)
	}
}

func TestRender_ReactPromptShowsCandidates(t *testing.T) {
	m := reactModel()
	m, _ = m.handleKey("r")
	for _, ch := range []string{"f", "i", "r", "e"} {
		m, _ = m.handleReactKey(ch, ch)
	}
	out := m.render()
	if !strings.Contains(out, "fire█ → [🔥]") {
		t.Errorf("react prompt missing bracketed first candidate: %q", out)
	}
}
