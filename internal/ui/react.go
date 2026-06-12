package ui

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/forPelevin/gomoji"

	"github.com/taziksh/beeper-tui/internal/api"
)

// defaultReactions are the quick-pick fallbacks when a network doesn't
// restrict its reaction set.
var defaultReactions = []string{"👍", "❤️", "😂", "😮", "😢"}

// maxReactSlots caps the picker at single-digit selection.
const maxReactSlots = 9

// cursorMessage returns the message under the conversation cursor, or nil.
func (m Model) cursorMessage() *api.Message {
	if m.msgSelected < 0 || m.msgSelected >= len(m.messages) {
		return nil
	}
	return &m.messages[m.msgSelected]
}

// selfUserID returns the user's own participant ID on the current chat's
// account, or "" when unknown.
func (m Model) selfUserID() string {
	idx := chatIndexByID(m.chats, m.currentChatID)
	if idx < 0 {
		return ""
	}
	return m.selfUsers[m.chats[idx].AccountID]
}

// reactSlots returns the numbered quick-pick reaction keys for the cursor
// message: the message's existing reactions first, so a digit joins or leaves
// them, then presets from the network's allowed set or the defaults.
func (m Model) reactSlots() []string {
	slots := make([]string, 0, maxReactSlots)
	seen := make(map[string]bool)
	if msg := m.cursorMessage(); msg != nil {
		for _, r := range msg.Reactions {
			if len(slots) >= maxReactSlots {
				return slots
			}
			if !seen[r.Key] {
				seen[r.Key] = true
				slots = append(slots, r.Key)
			}
		}
	}
	presets := defaultReactions
	if idx := chatIndexByID(m.chats, m.currentChatID); idx >= 0 && len(m.chats[idx].AllowedReactions) > 0 {
		presets = m.chats[idx].AllowedReactions
	}
	for _, k := range presets {
		if len(slots) >= maxReactSlots {
			break
		}
		if !seen[k] {
			seen[k] = true
			slots = append(slots, k)
		}
	}
	return slots
}

// openReactPicker enters REACT mode targeting the cursor message.
func (m Model) openReactPicker() Model {
	if len(m.messages) == 0 {
		return m
	}
	m.mode = ModeReact
	m.reactInput = ""
	m.reactCandIdx = 0
	m.reactErr = nil
	return m
}

// maxEmojiCandidates caps the fuzzy-search suggestions shown in the picker.
const maxEmojiCandidates = 5

// handleReactKey processes keys in REACT mode. A digit with an empty input
// toggles that numbered slot; anything typed fuzzy-searches emoji by name,
// tab cycles the matches, and enter sends the selected match or, with no
// match, the raw text.
func (m Model) handleReactKey(key, text string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = ModeConversation
		m.reactInput = ""
		m.reactCandIdx = 0
		return m, nil
	case "enter":
		k := strings.TrimSpace(m.reactInput)
		if k == "" {
			return m, nil
		}
		if cands := emojiCandidates(k, maxEmojiCandidates); len(cands) > 0 {
			return m.toggleReaction(cands[m.reactCandIdx%len(cands)])
		}
		return m.toggleReaction(k)
	case "tab":
		if n := len(emojiCandidates(m.reactInput, maxEmojiCandidates)); n > 0 {
			m.reactCandIdx = (m.reactCandIdx + 1) % n
		}
		return m, nil
	case "shift+tab":
		if n := len(emojiCandidates(m.reactInput, maxEmojiCandidates)); n > 0 {
			m.reactCandIdx = (m.reactCandIdx + n - 1) % n
		}
		return m, nil
	case "backspace":
		if r := []rune(m.reactInput); len(r) > 0 {
			m.reactInput = string(r[:len(r)-1])
		}
		m.reactCandIdx = 0
		return m, nil
	default:
		if m.reactInput == "" && len(text) == 1 && text[0] >= '1' && text[0] <= '9' {
			slots := m.reactSlots()
			if n := int(text[0] - '1'); n < len(slots) {
				return m.toggleReaction(slots[n])
			}
			return m, nil
		}
		m.reactInput += text
		m.reactCandIdx = 0
		return m, nil
	}
}

type emojiEntry struct{ slug, char string }

var (
	emojiTableOnce sync.Once
	emojiTable     []emojiEntry
)

// emojiEntries returns the searchable emoji table, built once: deduped by
// slug, skin-tone variants skipped, sorted by slug length then alphabetically
// so tier scans inside emojiCandidates need no per-call sorting.
func emojiEntries() []emojiEntry {
	emojiTableOnce.Do(func() {
		seen := make(map[string]bool)
		for _, e := range gomoji.AllEmojis() {
			if seen[e.Slug] || strings.Contains(e.Slug, "skin-tone") {
				continue
			}
			seen[e.Slug] = true
			emojiTable = append(emojiTable, emojiEntry{e.Slug, e.Character})
		}
		sort.Slice(emojiTable, func(i, j int) bool {
			if len(emojiTable[i].slug) != len(emojiTable[j].slug) {
				return len(emojiTable[i].slug) < len(emojiTable[j].slug)
			}
			return emojiTable[i].slug < emojiTable[j].slug
		})
	})
	return emojiTable
}

// emojiCandidates fuzzy-matches a typed query against emoji name slugs:
// exact match first, then prefix, then substring, shortest slug first within
// each tier. Returns nil for an empty query or one that already contains an
// emoji.
func emojiCandidates(query string, max int) []string {
	q := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(query)), " ", "-")
	if q == "" || gomoji.ContainsEmoji(q) {
		return nil
	}
	var exact, prefix, substr []string
	for _, e := range emojiEntries() {
		switch {
		case e.slug == q:
			exact = append(exact, e.char)
		case strings.HasPrefix(e.slug, q):
			prefix = append(prefix, e.char)
		case strings.Contains(e.slug, q):
			substr = append(substr, e.char)
		}
	}
	out := make([]string, 0, max)
	for _, tier := range [][]string{exact, prefix, substr} {
		for _, c := range tier {
			if len(out) >= max {
				return out
			}
			out = append(out, c)
		}
	}
	return out
}

// toggleReaction adds key to the cursor message, or removes it when the
// user's own reaction with that key is already there. The local message
// updates optimistically; a failed request reloads the chat to resync.
func (m Model) toggleReaction(key string) (Model, tea.Cmd) {
	msg := m.cursorMessage()
	if msg == nil {
		return m, nil
	}
	self := m.selfUserID()
	remove := false
	if self != "" {
		for _, r := range msg.Reactions {
			if r.Key == key && r.ParticipantID == self {
				remove = true
				break
			}
		}
	}
	if remove {
		kept := msg.Reactions[:0]
		for _, r := range msg.Reactions {
			if !(r.Key == key && r.ParticipantID == self) {
				kept = append(kept, r)
			}
		}
		msg.Reactions = kept
	} else {
		msg.Reactions = append(msg.Reactions, api.Reaction{Key: key, Emoji: looksEmoji(key), ParticipantID: self})
	}
	m.mode = ModeConversation
	m.reactInput = ""
	return m, m.reactCmd(m.currentChatID, msg.ID, key, remove)
}

// looksEmoji guesses whether a typed reaction key is an emoji rather than a
// shortcode. Only used for optimistic rendering; the server's answer arrives
// with the next update.
func looksEmoji(key string) bool {
	for _, r := range key {
		if r > 0x2000 {
			return true
		}
	}
	return false
}

type reactResultMsg struct {
	chatID string
	err    error
}

type selfUsersLoadedMsg struct{ users map[string]string }

func (m Model) reactCmd(chatID, messageID, key string, remove bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if remove {
			err = client.RemoveReaction(ctx, chatID, messageID, key)
		} else {
			err = client.AddReaction(ctx, chatID, messageID, key)
		}
		return reactResultMsg{chatID: chatID, err: err}
	}
}

// loadSelfUsersCmd fetches the user's own per-account IDs, best-effort: when
// it fails, reactions still send but toggling and the own-reaction marker are
// unavailable.
func (m Model) loadSelfUsersCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		users, err := client.SelfUserIDs(ctx)
		if err != nil {
			return selfUsersLoadedMsg{}
		}
		return selfUsersLoadedMsg{users: users}
	}
}
