package ui

import (
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/llm"
	"github.com/taziksh/beeper-tui/internal/person"
	"github.com/taziksh/beeper-tui/internal/ws"
)

// Mode is the top-level UI state. INSERT (M2) and overlays (M1.5) slot in later.
type Mode int

const (
	ModeList Mode = iota
	ModeConversation
	ModeInsert
	ModeSearch
	ModeReact
	ModeChat
	ModeChatInsert
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

	// warm-start cache state. An empty cachePath disables cache writes.
	cachePath    string
	cacheSavedAt time.Time

	// list state
	chats    []api.Chat
	tab      Tab // the selected tab
	selected int
	offset   int // first visible row in the list
	listPos  int // visible position when a chat was opened, restores the cursor on return

	// preview pane state
	previewOn    bool
	previewCache map[string][]api.Message // recent messages keyed by chat ID
	previewErr   map[string]error

	// conversation state
	currentChatID string
	messages      []api.Message
	msgOffset     int // first visible message row
	msgSelected   int // index of the cursor message

	// react picker state (REACT mode)
	reactInput   string
	reactCandIdx int // selected fuzzy-search candidate, cycled with tab
	reactErr     error
	selfUsers    map[string]string // own user ID per account ID, for recognizing own reactions

	// media state
	mediaPreviews map[string]mediaPreview // rendered inline previews keyed by attachment
	mediaErr      error                   // open/preview error, shown in the conversation status bar

	// compose state (INSERT mode)
	input       string
	composeAtts []string              // local file paths attached to the draft, sent on enter
	composeErr  error                 // clipboard/attach error, shown in the INSERT status bar
	failedSends map[string]failedSend // errored optimistic sends keyed by local id
	localSeq    int                   // mints local ids for optimistic messages

	// chat search state
	searchQuery    string
	searchResults  []api.MessageSearchResult
	searchSelected int
	searchOffset   int
	searchLoading  bool
	searchErr      error

	// chat tab (local-LLM assistant) state
	llm           *llm.Client
	people        *person.Store
	chatModel     string // model id shown in the status bar, "" until detected
	chatDetecting bool
	chatChecked   bool // endpoint has been checked at least once this session
	chatErr       error
	chatTurns     []chatTurn
	chatInput     string
	chatSession   *chatSession
	chatOffset    int  // first visible transcript line
	chatFollow    bool // pinned to the transcript bottom while streaming
	chatTokens    int  // streamed tokens in the current response
	chatStarted   time.Time
	chatLinks     []chatLink
	chatLinkSel   int // selected tappable name, -1 when none
	chatReasoning int // hidden reasoning tokens in the current response
	// returnToChat is set when a conversation was opened from a Chat-tab
	// tappable link; q/esc then restores ModeChat instead of the inbox list.
	returnToChat bool

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
	return Model{client: client, events: events, mode: ModeList, loadingChats: true, failedSends: map[string]failedSend{}, chatFollow: true, chatLinkSel: -1}
}

// WithLLM attaches the local-model client backing the chat tab. A nil client
// leaves the tab in its setup-help state.
func (m Model) WithLLM(c *llm.Client) Model {
	m.llm = c
	return m
}

// WithPeople attaches the person-card store used by the chat tools.
func (m Model) WithPeople(s *person.Store) Model {
	m.people = s
	return m
}

// failedSend records why an optimistic send errored, plus the draft needed
// to retry it with R.
type failedSend struct {
	reason string
	text   string
	atts   []string
}
