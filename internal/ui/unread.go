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

// accentColor signals "unread" — ANSI "3" (yellow) respects the user's terminal
// theme; lipgloss renders it as plain text under `go test` (no TTY).
var accentColor = lipgloss.Color("3")

// accentStyle is the accent applied on its own (e.g. message markers).
var accentStyle = lipgloss.NewStyle().Foreground(accentColor)

// sortChats orders chats pinned-first, then most-recent-first.
func sortChats(chats []api.Chat) {
	sort.SliceStable(chats, func(i, j int) bool {
		if chats[i].Pinned != chats[j].Pinned {
			return chats[i].Pinned
		}
		return chats[i].LastActive.After(chats[j].LastActive)
	})
}

// firstUnreadIndex returns the index of the earliest unread message, or -1 if
// none are unread.
func firstUnreadIndex(msgs []api.Message) int {
	for i, msg := range msgs {
		if msg.IsUnread {
			return i
		}
	}
	return -1
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
