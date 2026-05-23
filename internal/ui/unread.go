package ui

import (
	"sort"

	"charm.land/lipgloss/v2"

	"github.com/taziksh/beeper-tui/internal/api"
)

// unreadGlyph marks an unread chat row; readGlyph keeps columns aligned.
const (
	unreadGlyph = "●"
	readGlyph   = " "
	msgMarker   = "▎" // left bar on an unread message row
)

// accentStyle colors unread indicators. ANSI "3" (yellow) respects the user's
// terminal theme; lipgloss renders it as plain text under `go test` (no TTY).
var accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

// sortChats floats unread chats to the top, most-recent-first within each
// group.
func sortChats(chats []api.Chat) {
	sort.SliceStable(chats, func(i, j int) bool {
		iUnread, jUnread := chats[i].Unread > 0, chats[j].Unread > 0
		if iUnread != jUnread {
			return iUnread // unread (true) sorts before read (false)
		}
		return chats[i].LastActive.After(chats[j].LastActive)
	})
}

// reselectByID returns the index of the chat with id after a re-sort, or 0 if
// it's no longer present (e.g. filtered away). Callers clamp as needed.
func reselectByID(chats []api.Chat, id string) int {
	for i, c := range chats {
		if c.ID == id {
			return i
		}
	}
	return 0
}
