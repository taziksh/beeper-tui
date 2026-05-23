package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	switch m.mode {
	case ModeConversation, ModeInsert:
		return m.renderConversation()
	default:
		return m.renderList()
	}
}

func (m Model) renderList() string {
	if m.loadingChats {
		return "Loading chats…\n"
	}
	sel := lipgloss.NewStyle().Bold(true)
	var b strings.Builder
	b.WriteString("CHATS\n")
	vr := m.visibleRows()
	end := m.offset + vr
	if end > len(m.chats) {
		end = len(m.chats)
	}
	for i := m.offset; i < end; i++ {
		c := m.chats[i]
		mark := readGlyph
		count := fmt.Sprintf("%4d", c.Unread)
		if c.Unread > 0 {
			mark = accentStyle.Render(unreadGlyph)
			count = accentStyle.Render(count)
		}
		line := fmt.Sprintf("%s [%-10s] %s  %s", mark, truncate(c.Network, 10), count, c.Title)
		if i == m.selected {
			line = sel.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(m.statusBar())
	return b.String()
}

func (m Model) statusBar() string {
	return fmt.Sprintf("NORMAL  %d chats · j/k move · enter open · q quit", len(m.chats))
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
	for i := m.msgOffset; i < end; i++ {
		msg := m.messages[i]
		who := msg.SenderName
		if msg.IsFromMe {
			who = "You"
		}
		ts := msg.Timestamp.Format("15:04")
		line := fmt.Sprintf("%s  %-12s  %s", ts, truncate(who, 12), msg.Text)
		if m.failedSends[msg.ID] {
			line += "  ! send failed"
		}
		b.WriteString(line + "\n")
	}
	if m.mode == ModeInsert {
		b.WriteString("> " + m.input + "█\n")
	}
	b.WriteString(m.convStatusBar())
	return b.String()
}

func (m Model) convStatusBar() string {
	if m.mode == ModeInsert {
		return "INSERT  enter send · esc cancel"
	}
	return "NORMAL  j/k scroll · i reply · esc back · q quit"
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
