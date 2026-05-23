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

// isActiveUnread reports whether a chat should float to the top: it has unread
// messages and isn't muted or low-priority (those are noise the user has
// already deprioritized, so they stay in recency order instead of jumping up).
func isActiveUnread(c api.Chat) bool {
	return c.Unread > 0 && !c.Muted && !c.LowPriority
}

// sortChats floats unread chats to the top, most-recent-first within each
// group.
func sortChats(chats []api.Chat) {
	sort.SliceStable(chats, func(i, j int) bool {
		iUnread, jUnread := isActiveUnread(chats[i]), isActiveUnread(chats[j])
		if iUnread != jUnread {
			return iUnread // active unread floats above everything else
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
