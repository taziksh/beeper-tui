package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/taziksh/beeper-tui/internal/api"
)

// previewLoadedMsg is the preview-pane counterpart of messagesLoadedMsg; it
// never touches conversation state.
type previewLoadedMsg struct {
	chatID   string
	messages []api.Message
	err      error
}

func (m Model) togglePreview() (Model, tea.Cmd) {
	if m.mode != ModeList {
		return m, nil
	}
	m.previewOn = !m.previewOn
	if !m.previewOn {
		return m, nil
	}
	return m, m.previewLoad()
}

// previewLoad returns nil when the pane is off or the chat is already cached,
// so movement keys can call it unconditionally.
func (m Model) previewLoad() tea.Cmd {
	if !m.previewOn || m.mode != ModeList {
		return nil
	}
	if m.selected < 0 || m.selected >= len(m.chats) {
		return nil
	}
	chatID := m.chats[m.selected].ID
	if _, ok := m.previewCache[chatID]; ok {
		return nil
	}
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		msgs, err := client.ListMessages(ctx, chatID)
		return previewLoadedMsg{chatID: chatID, messages: msgs, err: err}
	}
}

func (m Model) applyPreviewLoaded(msg previewLoadedMsg) Model {
	if msg.err != nil {
		if m.previewErr == nil {
			m.previewErr = map[string]error{}
		}
		m.previewErr[msg.chatID] = msg.err
		return m
	}
	if m.previewCache == nil {
		m.previewCache = map[string][]api.Message{}
	}
	m.previewCache[msg.chatID] = msg.messages
	delete(m.previewErr, msg.chatID)
	return m
}

// joinPreview returns the rows unchanged when the terminal is too narrow to
// fit both panes.
func (m Model) joinPreview(left []string) []string {
	const sep = " │ "
	leftW := m.width / 2
	rightW := m.width - leftW - lipgloss.Width(sep)
	if leftW < 20 || rightW < 20 {
		return left
	}
	vr := m.visibleRows()
	right := m.previewLines(rightW, vr)
	clip := lipgloss.NewStyle().MaxWidth(leftW)
	out := make([]string, 0, vr)
	for i := 0; i < vr; i++ {
		var l, r string
		if i < len(left) {
			l = clip.Render(left[i])
		}
		if i < len(right) {
			r = right[i]
		}
		pad := leftW - lipgloss.Width(l)
		if pad < 0 {
			pad = 0
		}
		out = append(out, l+strings.Repeat(" ", pad)+sep+r)
	}
	return out
}

func (m Model) previewLines(w, maxRows int) []string {
	if m.selected < 0 || m.selected >= len(m.chats) {
		return nil
	}
	c := m.chats[m.selected]
	clip := lipgloss.NewStyle().MaxWidth(w)
	lines := []string{clip.Render(c.Title), ""}
	if err := m.previewErr[c.ID]; err != nil {
		return append(lines, clip.Render(fmt.Sprintf("preview failed: %v", err)))
	}
	msgs, ok := m.previewCache[c.ID]
	if !ok {
		return append(lines, "loading…")
	}
	if len(msgs) == 0 {
		return append(lines, "(no messages)")
	}
	avail := maxRows - len(lines)
	if avail < 1 {
		avail = 1
	}
	start := len(msgs) - avail
	if start < 0 {
		start = 0
	}
	for _, msg := range msgs[start:] {
		text := strings.ReplaceAll(msg.Text, "\n", " ")
		ts := formatMessageTime(msg.Timestamp, time.Now())
		lines = append(lines, clip.Render(fmt.Sprintf("%s %s: %s", ts, styledUsername(msg), text)))
	}
	return lines
}
