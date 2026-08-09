package api

import "time"

// Chat is our decoupled view of a Beeper chat — only the fields the TUI needs.
type Chat struct {
	ID               string
	AccountID        string
	Network          string // human label: "WhatsApp", "Signal", "iMessage"
	Title            string
	Type             string // "single" | "group" | etc.
	Unread           int
	Mentions         int // unread messages that @-mention the user
	Muted            bool
	LowPriority      bool
	Pinned           bool
	Archived         bool
	MarkedUnread     bool // user manually flagged the chat as unread
	LastActive       time.Time
	Preview          string        // plain-text last-message preview, may be empty
	LastFromMe       bool          // the preview message was sent by the authenticated user
	LastSender       string        // sender name of the preview message, may be empty
	AllowedReactions []string      // network's allowed reaction keys; empty means unrestricted
	PreviewSenderID  string        // raw sender ID of the preview message, for self-detection
	Participants     []Participant // may be a subset on large group chats
}

// Participant is a member of a chat.
type Participant struct {
	UserID   string
	FullName string
	Username string
	IsSelf   bool
	IsBot    bool // automated network account, e.g. a notification channel
}

// Contact is a person found via account contact search.
type Contact struct {
	AccountID   string
	UserID      string
	FullName    string
	Username    string
	PhoneNumber string
	Email       string
}

// Message is our decoupled view of a single message.
type Message struct {
	ID          string
	ChatID      string
	SenderName  string
	Text        string
	Timestamp   time.Time
	IsFromMe    bool
	IsUnread    bool // true if unread for the authenticated user; may be absent on some networks
	IsReaction  bool // a reaction event, not a real message; Desktop hides these in the timeline
	Reactions   []Reaction
	Attachments []Attachment
}

// Attachment is a media file carried by a message.
type Attachment struct {
	Type          string // "img" | "video" | "audio" | "unknown"
	ID            string // mxc:// identifier; resolve to a local path with DownloadAsset
	SrcURL        string // public URL or local file path, may be temporary
	FileName      string
	FileSize      int64   // bytes, 0 if unknown
	Duration      float64 // seconds, audio/video only
	MimeType      string
	Width         int
	Height        int
	IsVoiceNote   bool
	IsGif         bool
	IsSticker     bool
	Transcription string // voice-note transcription if the bridge provides one
}

// Reaction is one participant's reaction to a message.
type Reaction struct {
	Key           string // an emoji like 😄, or a network shortcode like "smiling-face"
	Emoji         bool   // true if Key is an emoji
	ParticipantID string // user ID of who reacted; matches SelfUserIDs values for own reactions
}

// MessageSearchResult is a message hit returned by content search.
type MessageSearchResult struct {
	Message Message
}
