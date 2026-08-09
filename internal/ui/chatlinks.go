package ui

import (
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/identity"
)

// chatLink is a person or chat the assistant mentioned, tappable to open
// the conversation.
type chatLink struct {
	name   string
	chatID string
}

// findChatLinks scans answer text for known people and chat titles and
// returns one link per distinct conversation, in first-appearance order.
func findChatLinks(text string, chats []api.Chat) []chatLink {
	if strings.TrimSpace(text) == "" || len(chats) == 0 {
		return nil
	}
	lastActive := make(map[string]time.Time, len(chats))
	for _, c := range chats {
		lastActive[c.ID] = c.LastActive
	}
	type hit struct {
		pos  int
		link chatLink
	}
	low := strings.ToLower(text)
	var hits []hit
	seen := map[string]bool{}
	add := func(name, chatID string) {
		if len(name) < 3 || chatID == "" || seen[chatID] {
			return
		}
		pos := wordIndex(low, strings.ToLower(name))
		if pos < 0 {
			return
		}
		seen[chatID] = true
		hits = append(hits, hit{pos: pos, link: chatLink{name: name, chatID: chatID}})
	}
	for _, p := range identity.Build(chats, nil).All() {
		add(p.Name, newestChat(p.Chats, lastActive))
	}
	for _, c := range chats {
		add(c.Title, c.ID)
	}
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].pos < hits[j-1].pos; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	links := make([]chatLink, len(hits))
	for i, h := range hits {
		links[i] = h.link
	}
	return links
}

// newestChat picks the most recently active of a person's chats.
func newestChat(refs []identity.ChatRef, lastActive map[string]time.Time) string {
	best := ""
	var bestAt time.Time
	for _, r := range refs {
		if at := lastActive[r.ID]; best == "" || at.After(bestAt) {
			best, bestAt = r.ID, at
		}
	}
	return best
}

// wordIndex finds needle in haystack at a word boundary, or -1.
func wordIndex(haystack, needle string) int {
	from := 0
	for {
		i := strings.Index(haystack[from:], needle)
		if i < 0 {
			return -1
		}
		i += from
		before := i == 0 || !isWordByte(haystack[i-1])
		after := i+len(needle) >= len(haystack) || !isWordByte(haystack[i+len(needle)])
		if before && after {
			return i
		}
		from = i + 1
	}
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

var (
	linkStyle    = lipgloss.NewStyle().Underline(true)
	linkSelStyle = lipgloss.NewStyle().Reverse(true)
)

// highlightChatLinks underlines every linked name in the rendered lines and
// inverts the selected one. It runs after wrapping, so the added ANSI codes
// cannot disturb layout; a name split across two lines just loses its
// highlight.
func (m Model) highlightChatLinks(lines []string) []string {
	if len(m.chatLinks) == 0 {
		return lines
	}
	for li, link := range m.chatLinks {
		style := linkStyle
		if li == m.chatLinkSel {
			style = linkSelStyle
		}
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(link.name) + `\b`)
		if err != nil {
			continue
		}
		for i := range lines {
			lines[i] = re.ReplaceAllStringFunc(lines[i], func(s string) string {
				return style.Render(s)
			})
		}
	}
	return lines
}

// openChatByID jumps from the chat tab into a conversation. q returns to the
// Chat tab (returnToChat), not the inbox list.
func (m Model) openChatByID(chatID string) (Model, tea.Cmd) {
	for i, c := range m.chats {
		if c.ID != chatID {
			continue
		}
		m.selected = i
		m.returnToChat = true
		// Keep TabChat selected so the bar shows origin; back restores ModeChat.
		return m.openSelected()
	}
	return m, nil
}
