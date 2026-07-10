package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/taziksh/beeper-tui/internal/api"
)

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	if debugLog != nil {
		defer logSlow("render", time.Now())
	}
	if m.err != nil {
		return m.renderOnboarding()
	}
	switch m.mode {
	case ModeConversation, ModeInsert, ModeReact:
		return m.renderConversation()
	case ModeSearch:
		return m.renderSearch()
	case ModeIdentity:
		return m.renderIdentity()
	default:
		return m.renderList()
	}
}

func (m Model) renderList() string {
	if m.loadingChats {
		return "Loading chats…\n"
	}
	var b strings.Builder
	b.WriteString(m.tabBar() + "\n")
	rows := m.listRows()
	if m.previewOn {
		rows = m.joinPreview(rows)
	}
	for _, row := range rows {
		b.WriteString(row + "\n")
	}
	b.WriteString(m.statusBar())
	return b.String()
}

func (m Model) listRows() []string {
	vr := m.visibleRows()
	indexes := m.visibleChatIndexes()
	rows := make([]string, 0, vr)
	for pos := m.offset; pos < len(indexes) && len(rows) < vr; pos++ {
		i := indexes[pos]
		c := m.chats[i]
		// base carries selection (bold); accent adds the unread color on top, so
		// a selected unread row renders both bold AND colored.
		base := lipgloss.NewStyle()
		if i == m.selected {
			base = base.Bold(true)
		}
		accent := base
		mark := readGlyph
		if c.Unread > 0 || c.MarkedUnread {
			accent = accent.Foreground(accentColor)
			mark = unreadGlyph
		}
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}
		line := base.Render(prefix) + accent.Render(mark) +
			base.Render(" ") + networkGlyph(c.Network) + base.Render(" ") +
			accent.Render(fmt.Sprintf("%4d", c.Unread)) +
			base.Render("  "+c.Title)
		rows = append(rows, line)
	}
	if len(indexes) == 0 {
		rows = append(rows, "  (empty)")
	}
	return rows
}

func (m Model) statusBar() string {
	if m.archiveErr != nil {
		return fmt.Sprintf("NORMAL  archive failed: %v", m.archiveErr)
	}
	if m.archivingChatID != "" {
		return "NORMAL  working…"
	}
	archive := "a archive"
	if m.tab == TabArchive {
		archive = "a unarchive"
	}
	return fmt.Sprintf("NORMAL  %sh/l tab · j/k · enter open · I notes · p preview · %s · / search · q quit", m.connStatus(), archive)
}

// renderOnboarding is the full-screen state when the chat list cannot load,
// pointing at the Desktop API setup steps instead of trapping the user in a
// bare error.
func (m Model) renderOnboarding() string {
	var b strings.Builder
	b.WriteString("Can't reach Beeper Desktop.\n\n")
	b.WriteString("  1. Open Beeper Desktop and keep it running\n")
	b.WriteString("  2. Settings → Developers → enable the Desktop API\n")
	b.WriteString("  3. export BEEPER_ACCESS_TOKEN=<your token>\n\n")
	b.WriteString(wrap(fmt.Sprintf("Error: %v", m.err), m.width) + "\n\n")
	b.WriteString("r retry · ctrl+c quit\n")
	return b.String()
}

func (m Model) renderSearch() string {
	var b strings.Builder
	b.WriteString("SEARCH /" + m.searchQuery + "\n")
	if m.searchErr != nil {
		b.WriteString(wrap(fmt.Sprintf("Error searching messages: %v", m.searchErr), m.width) + "\n")
		b.WriteString(m.searchStatusBar())
		return b.String()
	}
	if m.searchLoading {
		b.WriteString("Searching messages…\n")
		b.WriteString(m.searchStatusBar())
		return b.String()
	}
	vr := m.visibleRows()
	rows := 0
	for i := m.searchOffset; i < len(m.searchResults) && rows < vr; i++ {
		result := m.searchResults[i]
		msg := result.Message
		prefix := "  "
		if i == m.searchSelected {
			prefix = "> "
		}
		who := msg.SenderName
		if msg.IsFromMe {
			who = "You"
		}
		line := fmt.Sprintf("%s[%s] %-12s %s",
			prefix,
			truncate(m.chatTitle(msg.ChatID), 18),
			truncate(who, 12),
			msg.Text,
		)
		b.WriteString(line + "\n")
		rows++
	}
	if strings.TrimSpace(m.searchQuery) == "" {
		b.WriteString("Type to search messages\n")
	} else if len(m.searchResults) == 0 {
		b.WriteString("No matching messages\n")
	}
	b.WriteString(m.searchStatusBar())
	return b.String()
}

func (m Model) searchStatusBar() string {
	return "SEARCH  type query · enter open chat · esc clear"
}

func (m Model) chatTitle(id string) string {
	for _, c := range m.chats {
		if c.ID == id {
			return c.Title
		}
	}
	return id
}

func (m Model) renderConversation() string {
	var b strings.Builder
	b.WriteString(m.chatTitle(m.currentChatID) + "\n")
	if m.convErr != nil {
		b.WriteString(wrap(fmt.Sprintf("Error loading messages: %v", m.convErr), m.width) + "\n")
		b.WriteString(m.convStatusBar())
		return b.String()
	}
	if m.loadingMsgs {
		b.WriteString("Loading messages…\n")
		b.WriteString(m.convStatusBar())
		return b.String()
	}
	vr := m.visibleRows()
	end := m.msgOffset + vr
	if end > len(m.messages) {
		end = len(m.messages)
	}
	self := m.selfUserID()
	for i := m.msgOffset; i < end; i++ {
		msg := m.messages[i]
		who := msg.SenderName
		if msg.IsFromMe {
			who = "You"
		}
		ts := formatMessageTime(msg.Timestamp, time.Now())
		marker := readGlyph
		if msg.IsUnread {
			marker = accentStyle.Render(msgMarker)
		}
		prefix := "  "
		if i == m.msgSelected {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s %s  %-12s  %s", prefix, marker, ts, truncate(who, 12), msg.Text)
		if r := formatReactions(msg.Reactions, self); r != "" {
			line += "  " + r
		}
		if m.failedSends[msg.ID] {
			line += "  ! send failed"
		}
		b.WriteString(line + "\n")
	}
	if m.mode == ModeInsert {
		b.WriteString("> " + m.input + "█\n")
	}
	if m.mode == ModeReact {
		b.WriteString(m.reactPromptLine() + "\n")
	}
	b.WriteString(m.convStatusBar())
	return b.String()
}

// reactPromptLine shows the numbered quick-pick slots, the free-typed query,
// and its fuzzy-search matches with the tab-selected one bracketed.
func (m Model) reactPromptLine() string {
	parts := make([]string, 0, maxReactSlots)
	for i, k := range m.reactSlots() {
		parts = append(parts, fmt.Sprintf("%d %s", i+1, k))
	}
	line := "react: " + strings.Join(parts, "  ") + " · " + m.reactInput + "█"
	cands := emojiCandidates(m.reactInput, maxEmojiCandidates)
	if len(cands) == 0 {
		return line
	}
	sel := m.reactCandIdx % len(cands)
	marked := make([]string, len(cands))
	for i, c := range cands {
		if i == sel {
			marked[i] = "[" + c + "]"
		} else {
			marked[i] = c
		}
	}
	return line + " → " + strings.Join(marked, " ")
}

// formatReactions aggregates reactions into a compact suffix like "👍 2*  ❤️ 1",
// in order of first appearance. The trailing * marks a reaction that includes
// the user's own (selfID). Non-emoji keys render as a bracketed shortcode,
// e.g. "[smiling-face] 1". Returns "" when there are no reactions.
func formatReactions(reactions []api.Reaction, selfID string) string {
	if len(reactions) == 0 {
		return ""
	}
	counts := make(map[string]int, len(reactions))
	mine := make(map[string]bool, len(reactions))
	order := make([]string, 0, len(reactions))
	for _, r := range reactions {
		key := r.Key
		if !r.Emoji {
			key = "[" + key + "]"
		}
		if counts[key] == 0 {
			order = append(order, key)
		}
		counts[key]++
		if selfID != "" && r.ParticipantID == selfID {
			mine[key] = true
		}
	}
	parts := make([]string, 0, len(order))
	for _, key := range order {
		own := ""
		if mine[key] {
			own = "*"
		}
		parts = append(parts, fmt.Sprintf("%s %d%s", key, counts[key], own))
	}
	return strings.Join(parts, "  ")
}

func formatMessageTime(ts, now time.Time) string {
	if ts.IsZero() {
		return "--:--"
	}
	local := ts.In(now.Location())
	today := now.Local()
	if sameDay(local, today) {
		return local.Format("15:04")
	}
	if local.Year() == today.Year() {
		return local.Format("Jan 2 15:04")
	}
	return local.Format("2006-01-02 15:04")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func (m Model) convStatusBar() string {
	if m.mode == ModeInsert {
		return "INSERT  enter send · esc cancel"
	}
	if m.mode == ModeReact {
		return "REACT  1-9 toggle · type name · tab cycle · enter send · esc cancel"
	}
	if m.archiveErr != nil {
		return fmt.Sprintf("NORMAL  archive failed: %v", m.archiveErr)
	}
	if m.reactErr != nil {
		return fmt.Sprintf("NORMAL  react failed: %v", m.reactErr)
	}
	if m.archivingChatID != "" {
		return "NORMAL  archiving…"
	}
	return "NORMAL  " + m.connStatus() + "j/k · r react · i reply · I notes · a · q chats"
}

// wrap word-wraps s to width w so long errors stay fully readable instead of
// being truncated by the terminal edge. Width 0 (before the first WindowSizeMsg)
// returns the text unwrapped.
func wrap(s string, w int) string {
	if w <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(w).Render(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// tabBar renders the tab row, bracketing the active tab and badging Unread and
// Mentions with their counts.
func (m Model) tabBar() string {
	parts := make([]string, 0, len(tabOrder))
	for _, t := range tabOrder {
		text := t.label()
		if t == TabUnread || t == TabMentions {
			if n := t.count(m.chats); n > 0 {
				text = fmt.Sprintf("%s·%d", text, n)
			}
		}
		style := lipgloss.NewStyle()
		if t == m.tab {
			text = "[" + text + "]"
			style = style.Bold(true).Foreground(accentColor)
		}
		parts = append(parts, style.Render(text))
	}
	return strings.Join(parts, "  ")
}
