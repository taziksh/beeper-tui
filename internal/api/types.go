package api

import "time"

// Chat is our decoupled view of a Beeper chat — only the fields the TUI needs.
type Chat struct {
	ID           string
	AccountID    string
	Network      string // human label: "WhatsApp", "Signal", "iMessage"
	Title        string
	Type         string // "single" | "group" | etc.
	Unread       int
	Mentions     int // unread messages that @-mention the user
	Muted        bool
	LowPriority  bool
	Pinned       bool
	Archived     bool
	MarkedUnread bool // user manually flagged the chat as unread
	LastActive   time.Time
	Preview      string // plain-text last-message preview, may be empty
}

// Message is our decoupled view of a single message.
type Message struct {
	ID         string
	ChatID     string
	SenderName string
	Text       string
	Timestamp  time.Time
	IsFromMe   bool
	IsUnread   bool // true if unread for the authenticated user; may be absent on some networks
}

// MessageSearchResult is a message hit returned by content search.
type MessageSearchResult struct {
	Message Message
}
