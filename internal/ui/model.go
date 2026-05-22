package ui

import (
	"github.com/taziksh/beeper-tui/internal/api"
)

// Mode is the top-level UI state. INSERT (M2) and overlays (M1.5) slot in later.
type Mode int

const (
	ModeList Mode = iota
	ModeConversation
)

// Model holds all TUI state. bubbletea passes it by value through Update, so
// navigation methods use value receivers and return a new Model.
type Model struct {
	client *api.Client

	mode Mode

	// list state
	chats    []api.Chat
	selected int
	offset   int // first visible row in the list

	// conversation state
	currentChatID string
	messages      []api.Message
	msgOffset     int // first visible message row

	width  int
	height int

	loadingChats bool
	loadingMsgs  bool
	err          error

	pendingG bool // tracks a pending `g` for the `gg` motion
}

// New builds the initial model. The chat fetch happens in Init, not here.
func New(client *api.Client) Model {
	return Model{client: client, mode: ModeList, loadingChats: true}
}
