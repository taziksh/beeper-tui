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

// chatTier orders chats into the list's three sections: active unread (0) floats
// to the top, normal read (1) sits in the middle, and deprioritized muted or
// low-priority chats (2) form a block at the bottom.
func chatTier(c api.Chat) int {
	switch {
	case c.Muted || c.LowPriority:
		return 2
	case c.Unread > 0:
		return 0
	default:
		return 1
	}
}

// lowPriorityStart returns the index of the first muted/low-priority chat in a
// sorted list (where the bottom section begins), or len(chats) if there are none.
func lowPriorityStart(chats []api.Chat) int {
	for i, c := range chats {
		if c.Muted || c.LowPriority {
			return i
		}
	}
	return len(chats)
}

// sortChats floats unread chats to the top, most-recent-first within each
// group, and pushes muted/low-priority chats to a contiguous bottom block.
func sortChats(chats []api.Chat) {
	sort.SliceStable(chats, func(i, j int) bool {
		ti, tj := chatTier(chats[i]), chatTier(chats[j])
		if ti != tj {
			return ti < tj // lower tier first: active unread, then read, then low-priority
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
