package ui

import "github.com/taziksh/beeper-tui/internal/api"

// Tab is a client-side filter over the fetched chat list. Each tab partitions
// chats by flags already present on every Chat, so switching tabs needs no
// network round-trip.
type Tab int

const (
	TabInbox Tab = iota
	TabUnread
	TabMentions
	TabPinned
	TabLowPriority
	TabArchive
)

// tabOrder is the left-to-right tab bar order, also the order h/l cycle through.
var tabOrder = []Tab{TabInbox, TabUnread, TabMentions, TabPinned, TabLowPriority, TabArchive}

// label is the tab's name as shown in the tab bar.
func (t Tab) label() string {
	switch t {
	case TabUnread:
		return "Unread"
	case TabMentions:
		return "Mentions"
	case TabPinned:
		return "Pinned"
	case TabLowPriority:
		return "Low Priority"
	case TabArchive:
		return "Archive"
	default:
		return "Inbox"
	}
}

// includes reports whether chat belongs in this tab. Archived chats live only in
// the Archive tab; every other tab is implicitly an active-inbox view.
func (t Tab) includes(c api.Chat) bool {
	if c.Archived {
		return t == TabArchive
	}
	switch t {
	case TabUnread:
		return c.Unread > 0 || c.MarkedUnread
	case TabMentions:
		return c.Mentions > 0
	case TabPinned:
		return c.Pinned
	case TabLowPriority:
		return c.LowPriority || c.Muted
	case TabArchive:
		return false
	default: // TabInbox
		return !c.LowPriority && !c.Muted
	}
}

// count returns how many chats fall in the tab, used for the tab bar badges.
func (t Tab) count(chats []api.Chat) int {
	n := 0
	for _, c := range chats {
		if t.includes(c) {
			n++
		}
	}
	return n
}
