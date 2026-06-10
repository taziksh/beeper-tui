package ui

import (
	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/ws"
)

// Mode is the top-level UI state. INSERT (M2) and overlays (M1.5) slot in later.
type Mode int

const (
	ModeList Mode = iota
	ModeConversation
	ModeInsert
	ModeSearch
)

// ConnState is the live-events connection state shown in the status bar.
// The zero value connIdle means no WebSocket client is attached and keeps the
// status bar quiet.
type ConnState int

const (
	connIdle ConnState = iota
	connConnecting
	connConnected
	connDisconnected
)

// Model holds all TUI state. bubbletea passes it by value through Update, so
// navigation methods use value receivers and return a new Model.
type Model struct {
	client *api.Client
	events *ws.Client

	mode Mode

	// live-events connection state
	conn          ConnState
	connErr       error
	everConnected bool // distinguishes a reconnect, which refetches, from first connect

	// list state
	chats    []api.Chat
	tab      Tab // the selected tab
	selected int
	offset   int // first visible row in the list

	// preview pane state
	previewOn    bool
	previewCache map[string][]api.Message // recent messages keyed by chat ID
	previewErr   map[string]error

	// conversation state
	currentChatID string
	messages      []api.Message
	msgOffset     int // first visible message row

	// compose state (INSERT mode)
	input       string
	failedSends map[string]bool // local ids of optimistic sends that errored
	localSeq    int             // mints local ids for optimistic messages

	// chat search state
	searchQuery    string
	searchResults  []api.MessageSearchResult
	searchSelected int
	searchOffset   int
	searchLoading  bool
	searchErr      error

	width  int
	height int

	loadingChats bool
	loadingMsgs  bool
	err          error // fatal chat-list load error (full-screen)
	convErr      error // conversation-load error, scoped to the conversation body
	archiveErr   error // archive error, scoped to the current list/conversation status

	archivingChatID string

	pendingG bool // tracks a pending `g` for the `gg` motion
}

// New builds the initial model. The chat fetch happens in Init, not here.
// events may be nil, which disables live updates.
func New(client *api.Client, events *ws.Client) Model {
	return Model{client: client, events: events, mode: ModeList, loadingChats: true, failedSends: map[string]bool{}}
}
