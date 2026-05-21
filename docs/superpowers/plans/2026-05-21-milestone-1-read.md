# Milestone 1 — Read Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Launch `beeper-tui` → reach a conversation → read its real messages on screen, vim-modal. The first thing the user can actually *do*.

**Architecture:** A new `internal/ui` package on bubbletea v2's Elm pattern (`Model`/`Init`/`Update`/`View`). A `Mode` field (`List` | `Conversation`) drives behavior. Chats and messages load asynchronously via `tea.Cmd` (UI never blocks; loading states shown). All navigation/state logic lives in **pure methods on `Model`** (`cursorDown`, `openSelected`, etc.), unit-tested without constructing bubbletea types; `Update` is a thin key→method dispatcher. Built on Phase 2's `internal/api` (`ListChats`/`ListMessages`/`MarkRead`).

**Tech Stack:** Go 1.26.3, bubbletea v2 (`charm.land/bubbletea/v2`), lipgloss v2 (`charm.land/lipgloss/v2`), existing `internal/api` + `internal/config`.

**Spec:** `docs/superpowers/specs/2026-05-21-beeper-tui-read-reply-design.md`

**Module:** `github.com/taziksh/beeper-tui`

---

## Scope

**In M1:** launch → chat list (loading state) → `j`/`k`/`gg`/`G` move selection + `Enter` open → conversation view with messages (sender/time/text) + `j`/`k`/`gg`/`G` scroll → `Esc` back to list → `:q` quit. Mark-as-read on open. NORMAL-mode + Mode-machine infrastructure so M2/M1.5 slot in.

**Deferred to M1.5 (fast-follow, NOT here):** `<leader>ff` fuzzy finder, the hotlist bar, toggleable list while in a conversation (`<leader>l`), `]u`/`[u` next/prev unread.

**Deferred to M2:** reply / compose / INSERT mode (we build the Mode field now, but no INSERT behavior yet).

---

## CRITICAL data rule

All automated tests use **synthetic/fake data only** (e.g. chat titles "Alice"/"Dev Team", messages "hello"). **Never** use the user's real Beeper data. Do **not** run `./beeper-tui` against the live API in a way that prints real chats/messages into the conversation — the **user** runs live verification themselves and reports pass/fail. The build-tagged integration test may hit the real API but asserts only counts/non-empty, never printing content.

---

## File Structure

| File | Task | Responsibility |
|---|---|---|
| `go.mod` / `go.sum` | 1 | bubbletea v2 + lipgloss v2 deps |
| `internal/ui/model.go` | 2 | `Model` struct, `Mode` type, `New()` |
| `internal/ui/messages.go` | 3 | `tea.Msg` types + load commands |
| `internal/ui/nav.go` | 5,6,7,9,10 | pure state methods (`cursorDown`, `openSelected`, `backToList`, `clampWindow`, …) |
| `internal/ui/update.go` | 3,4,8,11 | `Update` — thin key/msg → method dispatch + `Init` |
| `internal/ui/view.go` | 12,13,14 | `View` + render helpers (list, conversation, status bar) |
| `internal/ui/nav_test.go` | 5,6,7,9,10 | pure nav/state unit tests |
| `internal/ui/update_test.go` | 4,8,11 | reducer tests for msg handling |
| `internal/ui/view_test.go` | 12,13,14 | render substring tests |
| `internal/ui/integration_test.go` | 17 | build-tagged real-API test (counts only) |
| `cmd/beeper-tui/main.go` | 16 | launch the bubbletea program |

---

## Conventions

- Conventional Commits, one commit per task.
- Pure methods use value receiver, return `Model` — tests call them directly, never constructing `tea.Msg`.
- `Update` stays thin: translate `tea.KeyPressMsg.String()` → a pure method.
- Test names `Test<Method>_<Scenario>`.

---

## Task 1 — Add bubbletea v2 + lipgloss v2, spike a minimal program

**Files:** Modify `go.mod`, `go.sum`; create (temp) `internal/ui/spike/main.go`

- [ ] **Step 1: Add deps**

```bash
cd /Users/tazik/Projects/beeper-tui
go get charm.land/bubbletea/v2@latest
go get charm.land/lipgloss/v2@latest
```

- [ ] **Step 2: Minimal v2 program**

Create `internal/ui/spike/main.go`:

```go
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

type model struct{ w, h int; key string }

func (m model) Init() tea.Cmd { return nil }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyPressMsg:
		m.key = msg.String()
		if m.key == "q" || m.key == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}
func (m model) View() tea.View {
	return tea.NewView(fmt.Sprintf("size %dx%d key %q — q to quit\n", m.w, m.h, m.key))
}
func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/ui/spike`
Expected: clean. If `tea.NewView`, `tea.KeyPressMsg`, `tea.WindowSizeMsg.Width/Height`, or `tea.NewProgram(...).Run()` don't compile, run `go doc charm.land/bubbletea/v2.<symbol>` and adjust; **report any deviation.**

- [ ] **Step 4: Capture lipgloss v2 API**

Run: `go doc charm.land/lipgloss/v2.NewStyle` and `go doc charm.land/lipgloss/v2.Style` (list main methods: Foreground, Bold, Width, Padding, Render) and `go doc charm.land/lipgloss/v2.JoinVertical`. **Paste output in your report** — the View tasks need exact method names.

- [ ] **Step 5: Delete spike, commit deps**

```bash
rm -rf internal/ui/spike
git add go.mod go.sum
git commit -m "chore(ui): add bubbletea v2 + lipgloss v2 deps"
```

- [ ] **Step 6: Report** the confirmed bubbletea v2 signatures + lipgloss v2 style methods.

---

## Task 2 — Model struct + Mode machine

**Files:** Create `internal/ui/model.go`

- [ ] **Step 1: Write the model**

```go
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
}

func New(client *api.Client) Model {
	return Model{client: client, mode: ModeList, loadingChats: true}
}
```

- [ ] **Step 2: Build**

Run: `go build ./internal/ui/...`
Expected: clean (unused struct fields are fine in Go).

- [ ] **Step 3: Commit**

```bash
git add internal/ui/model.go
git commit -m "feat(ui): Model struct and Mode machine"
```

---

## Task 3 — Messages, load commands, Init + Update skeleton

**Files:** Create `internal/ui/messages.go`, `internal/ui/update.go`

- [ ] **Step 1: Write messages + commands**

Create `internal/ui/messages.go`:

```go
package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
)

type chatsLoadedMsg struct{ chats []api.Chat }
type messagesLoadedMsg struct {
	chatID   string
	messages []api.Message
}
type errMsg struct{ err error }

func (m Model) loadChatsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		chats, err := client.ListChats(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return chatsLoadedMsg{chats: chats}
	}
}

func (m Model) loadMessagesCmd(chatID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		msgs, err := client.ListMessages(ctx, chatID)
		if err != nil {
			return errMsg{err: err}
		}
		return messagesLoadedMsg{chatID: chatID, messages: msgs}
	}
}

func (m Model) markReadCmd(chatID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.MarkRead(ctx, chatID) // best-effort; ignore error for read state
		return nil
	}
}
```

- [ ] **Step 2: Write Init + Update skeleton**

Create `internal/ui/update.go`:

```go
package ui

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) Init() tea.Cmd {
	return m.loadChatsCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chatsLoadedMsg:
		m.chats = msg.chats
		m.loadingChats = false
		return m, nil
	case messagesLoadedMsg:
		if msg.chatID == m.currentChatID {
			m.messages = msg.messages
			m.loadingMsgs = false
		}
		return m, nil
	case errMsg:
		m.err = msg.err
		m.loadingChats = false
		m.loadingMsgs = false
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.clampWindow(), nil
	case tea.KeyPressMsg:
		return m.handleKey(msg.String())
	}
	return m, nil
}
```

- [ ] **Step 3: Build — expect failure**

Run: `go build ./internal/ui/...`
Expected: FAIL — `m.clampWindow` and `m.handleKey` undefined (added in Tasks 4–5). This is expected; proceed.

- [ ] **Step 4: Commit (compiles after Task 5; commit messages.go alone now)**

```bash
git add internal/ui/messages.go
git commit -m "feat(ui): load commands and message types"
```

(`update.go` is committed in Task 5 once `clampWindow`/`handleKey` exist and it builds.)

---

## Task 4 — Reducer test: chats/messages/error handling

**Files:** Create `internal/ui/update_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package ui

import (
	"errors"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestUpdate_ChatsLoaded(t *testing.T) {
	m := Model{loadingChats: true}
	got, _ := m.Update(chatsLoadedMsg{chats: []api.Chat{{ID: "a", Title: "Alice"}}})
	gm := got.(Model)
	if gm.loadingChats {
		t.Error("loadingChats should be false")
	}
	if len(gm.chats) != 1 || gm.chats[0].Title != "Alice" {
		t.Errorf("chats = %+v, want one 'Alice'", gm.chats)
	}
}

func TestUpdate_MessagesLoadedForCurrentChat(t *testing.T) {
	m := Model{currentChatID: "a", loadingMsgs: true}
	got, _ := m.Update(messagesLoadedMsg{chatID: "a", messages: []api.Message{{ID: "m1", Text: "hi"}}})
	gm := got.(Model)
	if gm.loadingMsgs {
		t.Error("loadingMsgs should be false")
	}
	if len(gm.messages) != 1 || gm.messages[0].Text != "hi" {
		t.Errorf("messages = %+v, want one 'hi'", gm.messages)
	}
}

func TestUpdate_MessagesIgnoredForStaleChat(t *testing.T) {
	m := Model{currentChatID: "a", loadingMsgs: true}
	got, _ := m.Update(messagesLoadedMsg{chatID: "OLD", messages: []api.Message{{ID: "x"}}})
	gm := got.(Model)
	if len(gm.messages) != 0 {
		t.Error("messages for a non-current chat must be ignored")
	}
}

func TestUpdate_Error(t *testing.T) {
	m := Model{loadingChats: true}
	got, _ := m.Update(errMsg{err: errors.New("boom")})
	gm := got.(Model)
	if gm.err == nil || gm.loadingChats {
		t.Error("error should be set and loading cleared")
	}
}
```

- [ ] **Step 2: Run — expect build failure**

Run: `go test ./internal/ui/...`
Expected: build fails (`clampWindow`/`handleKey` still undefined). That's fine — Task 5 makes it build. Do NOT commit yet; proceed to Task 5, then return and confirm these pass.

---

## Task 5 — List navigation: cursor down/up + clamp + key dispatch

**Files:** Create `internal/ui/nav.go`; finalize `internal/ui/update.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/nav_test.go`:

```go
package ui

import (
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func chats(n int) []api.Chat {
	cs := make([]api.Chat, n)
	for i := range cs {
		cs[i] = api.Chat{ID: string(rune('a' + i%26))}
	}
	return cs
}

func TestCursorDown_List_Advances(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(3), selected: 0, height: 10}
	m = m.cursorDown()
	if m.selected != 1 {
		t.Errorf("selected = %d, want 1", m.selected)
	}
}

func TestCursorDown_List_ClampsBottom(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(3), selected: 2, height: 10}
	m = m.cursorDown()
	if m.selected != 2 {
		t.Errorf("selected = %d, want clamped 2", m.selected)
	}
}

func TestCursorUp_List_ClampsTop(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(3), selected: 0, height: 10}
	m = m.cursorUp()
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0", m.selected)
	}
}

func TestCursorDown_EmptyList_NoPanic(t *testing.T) {
	m := Model{mode: ModeList, chats: nil, height: 10}
	m = m.cursorDown()
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0", m.selected)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestCursor`
Expected: build failure — `m.cursorDown`/`cursorUp` undefined.

- [ ] **Step 3: Implement nav.go**

Create `internal/ui/nav.go`:

```go
package ui

// visibleRows is how many rows the list/conversation body can show. Reserves
// two rows (header + status bar). Falls back before the first WindowSizeMsg.
func (m Model) visibleRows() int {
	r := m.height - 2
	if r < 1 {
		return 1
	}
	return r
}

func (m Model) cursorDown() Model {
	switch m.mode {
	case ModeList:
		if len(m.chats) > 0 && m.selected < len(m.chats)-1 {
			m.selected++
		}
	case ModeConversation:
		if m.msgOffset < m.maxMsgOffset() {
			m.msgOffset++
		}
	}
	return m.clampWindow()
}

func (m Model) cursorUp() Model {
	switch m.mode {
	case ModeList:
		if m.selected > 0 {
			m.selected--
		}
	case ModeConversation:
		if m.msgOffset > 0 {
			m.msgOffset--
		}
	}
	return m.clampWindow()
}

func (m Model) maxMsgOffset() int {
	max := len(m.messages) - m.visibleRows()
	if max < 0 {
		return 0
	}
	return max
}

func (m Model) clampWindow() Model {
	switch m.mode {
	case ModeList:
		vr := m.visibleRows()
		if m.selected < m.offset {
			m.offset = m.selected
		}
		if m.selected >= m.offset+vr {
			m.offset = m.selected - vr + 1
		}
		if m.offset < 0 {
			m.offset = 0
		}
	case ModeConversation:
		if m.msgOffset > m.maxMsgOffset() {
			m.msgOffset = m.maxMsgOffset()
		}
		if m.msgOffset < 0 {
			m.msgOffset = 0
		}
	}
	return m
}

// handleKey maps a key string to a pure method. Update stays thin.
func (m Model) handleKey(key string) (Model, teaCmd) {
	switch key {
	case "ctrl+c", "q", ":q":
		return m, quitCmd
	case "j", "down":
		return m.cursorDown(), nil
	case "k", "up":
		return m.cursorUp(), nil
	}
	return m, nil
}
```

**IMPORTANT for the implementer:** Two type fixes needed:
1. `teaCmd` is a placeholder — import `tea "charm.land/bubbletea/v2"` and use `tea.Cmd` as the return type: `func (m Model) handleKey(key string) (Model, tea.Cmd)`.
2. `quitCmd` placeholder → use `tea.Quit` (the bubbletea quit command). So the quit case is `return m, tea.Quit`.
The `":q"` case here is a placeholder for the quit key; the real `:` command-line is M1.5 — for M1, map bare `q` and `ctrl+c` to quit (drop the `":q"` literal, it won't arrive as a single key). Final quit case: `case "ctrl+c", "q": return m, tea.Quit`.

- [ ] **Step 4: Confirm update.go builds**

`update.go` (from Task 3) calls `m.clampWindow()` and `m.handleKey(...)`, now defined. Run:
```bash
go build ./internal/ui/...
```
Expected: clean.

- [ ] **Step 5: Run all ui tests**

Run: `go test ./internal/ui/... -v`
Expected: Task 4's reducer tests AND the cursor tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/nav.go internal/ui/update.go internal/ui/update_test.go internal/ui/nav_test.go
git commit -m "feat(ui): list navigation (j/k) + key dispatch + reducer"
```

---

## Task 6 — List navigation: gg/G

**Files:** Modify `internal/ui/nav.go`, `internal/ui/nav_test.go`, `internal/ui/update.go`

- [ ] **Step 1: Add failing tests**

Append to `internal/ui/nav_test.go`:

```go
func TestJumpBottom_List(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(5), selected: 0, height: 10}
	m = m.jumpBottom()
	if m.selected != 4 {
		t.Errorf("selected = %d, want 4", m.selected)
	}
}

func TestJumpTop_List(t *testing.T) {
	m := Model{mode: ModeList, chats: chats(5), selected: 4, offset: 3, height: 10}
	m = m.jumpTop()
	if m.selected != 0 || m.offset != 0 {
		t.Errorf("selected/offset = %d/%d, want 0/0", m.selected, m.offset)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestJump`
Expected: `jumpTop`/`jumpBottom` undefined.

- [ ] **Step 3: Implement**

Append to `internal/ui/nav.go`:

```go
func (m Model) jumpTop() Model {
	switch m.mode {
	case ModeList:
		m.selected = 0
	case ModeConversation:
		m.msgOffset = 0
	}
	return m.clampWindow()
}

func (m Model) jumpBottom() Model {
	switch m.mode {
	case ModeList:
		if len(m.chats) > 0 {
			m.selected = len(m.chats) - 1
		}
	case ModeConversation:
		m.msgOffset = m.maxMsgOffset()
	}
	return m.clampWindow()
}
```

- [ ] **Step 4: Wire keys** in `handleKey` (in nav.go). Add a pending-`g` field to `Model` in `model.go`: `pendingG bool`. Then in `handleKey`, reset it for non-`g` keys and handle `gg`/`G`:

```go
func (m Model) handleKey(key string) (Model, tea.Cmd) {
	if key != "g" {
		m.pendingG = false
	}
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "j", "down":
		return m.cursorDown(), nil
	case "k", "up":
		return m.cursorUp(), nil
	case "G":
		return m.jumpBottom(), nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			return m.jumpTop(), nil
		}
		m.pendingG = true
		return m, nil
	}
	return m, nil
}
```

- [ ] **Step 5: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/nav.go internal/ui/nav_test.go internal/ui/update.go internal/ui/model.go
git commit -m "feat(ui): gg/G jump in list"
```

---

## Task 7 — Open a chat (Enter): switch to Conversation, load messages, mark read

**Files:** Modify `internal/ui/nav.go`, `internal/ui/nav_test.go`, `internal/ui/update.go`

- [ ] **Step 1: Add failing test**

Append to `internal/ui/nav_test.go`:

```go
func TestOpenSelected_SwitchesMode(t *testing.T) {
	m := Model{mode: ModeList, chats: []api.Chat{{ID: "a", Title: "Alice"}}, selected: 0}
	m2, cmd := m.openSelected()
	if m2.mode != ModeConversation {
		t.Error("mode should be ModeConversation")
	}
	if m2.currentChatID != "a" {
		t.Errorf("currentChatID = %q, want a", m2.currentChatID)
	}
	if !m2.loadingMsgs {
		t.Error("loadingMsgs should be true while fetching")
	}
	if cmd == nil {
		t.Error("openSelected should return a command (load messages + mark read)")
	}
}

func TestOpenSelected_EmptyList_NoOp(t *testing.T) {
	m := Model{mode: ModeList, chats: nil}
	m2, cmd := m.openSelected()
	if m2.mode != ModeList || cmd != nil {
		t.Error("opening with no chats should do nothing")
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestOpenSelected`
Expected: `m.openSelected` undefined.

- [ ] **Step 3: Implement** in `internal/ui/nav.go`:

```go
import tea "charm.land/bubbletea/v2"

// openSelected enters the selected chat: switches mode, resets scroll,
// and returns a command that loads messages and marks the chat read.
func (m Model) openSelected() (Model, tea.Cmd) {
	if len(m.chats) == 0 || m.selected >= len(m.chats) {
		return m, nil
	}
	chat := m.chats[m.selected]
	m.mode = ModeConversation
	m.currentChatID = chat.ID
	m.messages = nil
	m.msgOffset = 0
	m.loadingMsgs = true
	return m, tea.Batch(m.loadMessagesCmd(chat.ID), m.markReadCmd(chat.ID))
}
```

(Add the `tea` import to nav.go if not already present. `tea.Batch` runs both commands.)

- [ ] **Step 4: Wire `Enter`** in `handleKey`, add to the switch:

```go
	case "enter":
		if m.mode == ModeList {
			return m.openSelected()
		}
		return m, nil
```

- [ ] **Step 5: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/nav.go internal/ui/nav_test.go internal/ui/update.go
git commit -m "feat(ui): open chat on enter, load messages, mark read"
```

---

## Task 8 — Esc back to list

**Files:** Modify `internal/ui/nav.go`, `internal/ui/nav_test.go`, `internal/ui/update.go`

- [ ] **Step 1: Add failing test**

Append to `internal/ui/nav_test.go`:

```go
func TestBackToList(t *testing.T) {
	m := Model{mode: ModeConversation, currentChatID: "a", messages: []api.Message{{ID: "m"}}}
	m = m.backToList()
	if m.mode != ModeList {
		t.Error("mode should be ModeList")
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestBackToList`
Expected: `m.backToList` undefined.

- [ ] **Step 3: Implement** in `nav.go`:

```go
func (m Model) backToList() Model {
	m.mode = ModeList
	return m
}
```

- [ ] **Step 4: Wire `esc`** in `handleKey`:

```go
	case "esc":
		if m.mode == ModeConversation {
			return m.backToList(), nil
		}
		return m, nil
```

- [ ] **Step 5: Run + commit**

Run: `go test ./internal/ui/... -v` (expect pass), then:
```bash
git add internal/ui/nav.go internal/ui/nav_test.go internal/ui/update.go
git commit -m "feat(ui): esc returns to list"
```

---

## Task 9 — Conversation scroll behaves (j/k/gg/G in Conversation mode)

The cursorDown/Up and jump methods already branch on `mode`. This task adds a test proving scroll works in Conversation mode with a realistic viewport.

**Files:** Modify `internal/ui/nav_test.go`

- [ ] **Step 1: Add tests**

Append to `internal/ui/nav_test.go`:

```go
func msgs(n int) []api.Message {
	ms := make([]api.Message, n)
	for i := range ms {
		ms[i] = api.Message{ID: string(rune('a' + i%26))}
	}
	return ms
}

func TestConversationScroll_DownAndClamp(t *testing.T) {
	// height 7 -> visibleRows 5. 20 messages -> maxMsgOffset 15.
	m := Model{mode: ModeConversation, messages: msgs(20), height: 7}
	for i := 0; i < 100; i++ {
		m = m.cursorDown()
	}
	if m.msgOffset != 15 {
		t.Errorf("msgOffset = %d, want clamped 15", m.msgOffset)
	}
}

func TestConversationScroll_JumpTopBottom(t *testing.T) {
	m := Model{mode: ModeConversation, messages: msgs(20), height: 7, msgOffset: 10}
	m = m.jumpTop()
	if m.msgOffset != 0 {
		t.Errorf("after jumpTop msgOffset = %d, want 0", m.msgOffset)
	}
	m = m.jumpBottom()
	if m.msgOffset != 15 {
		t.Errorf("after jumpBottom msgOffset = %d, want 15", m.msgOffset)
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/ui/... -run TestConversationScroll -v`
Expected: PASS (logic from Tasks 5–6 already handles Conversation mode). If it fails, fix `maxMsgOffset`/`clampWindow` until green.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/nav_test.go
git commit -m "test(ui): conversation scroll clamping"
```

---

## Task 10 — WindowSize re-clamps (regression guard)

**Files:** Modify `internal/ui/update_test.go`

- [ ] **Step 1: Add test**

Append to `internal/ui/update_test.go`:

```go
import tea "charm.land/bubbletea/v2"

func TestUpdate_WindowSizeReclamps(t *testing.T) {
	m := Model{mode: ModeConversation, messages: msgs(50), msgOffset: 40, height: 30}
	got, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 6})
	gm := got.(Model)
	if gm.height != 6 {
		t.Errorf("height = %d, want 6", gm.height)
	}
	if gm.msgOffset > gm.maxMsgOffset() {
		t.Errorf("msgOffset %d exceeds max %d after resize", gm.msgOffset, gm.maxMsgOffset())
	}
}
```

- [ ] **Step 2: Run + commit**

Run: `go test ./internal/ui/... -run TestUpdate_WindowSize -v` (expect PASS — `Update`'s WindowSizeMsg case already calls `clampWindow`). Then:
```bash
git add internal/ui/update_test.go
git commit -m "test(ui): window resize re-clamps scroll"
```

---

## Task 11 — View: chat list

**Files:** Create `internal/ui/view.go`, `internal/ui/view_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/ui/view_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestRender_LoadingChats(t *testing.T) {
	m := Model{mode: ModeList, loadingChats: true, width: 80, height: 24}
	if !strings.Contains(m.render(), "Loading") {
		t.Errorf("loading view missing 'Loading': %q", m.render())
	}
}

func TestRender_ListShowsTitles(t *testing.T) {
	m := Model{
		mode: ModeList, width: 80, height: 24,
		chats: []api.Chat{
			{ID: "a", Network: "Signal", Title: "Alice", Unread: 0},
			{ID: "b", Network: "WhatsApp", Title: "Dev Team", Unread: 5},
		},
		selected: 0,
	}
	out := m.render()
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Dev Team") {
		t.Errorf("list missing titles: %q", out)
	}
	if !strings.Contains(out, "5") {
		t.Errorf("list missing unread count: %q", out)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestRender`
Expected: `m.render` undefined.

- [ ] **Step 3: Implement view.go**

Create `internal/ui/view.go` (uses lipgloss v2 API confirmed in Task 1; adjust method names if Task 1 found differences):

```go
package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) View() tea.View {
	return tea.NewView(m.render())
}

func (m Model) render() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	switch m.mode {
	case ModeConversation:
		return m.renderConversation()
	default:
		return m.renderList()
	}
}

func (m Model) renderList() string {
	if m.loadingChats {
		return "Loading chats…\n"
	}
	sel := lipgloss.NewStyle().Bold(true)
	var b strings.Builder
	b.WriteString("CHATS\n")
	vr := m.visibleRows()
	end := m.offset + vr
	if end > len(m.chats) {
		end = len(m.chats)
	}
	for i := m.offset; i < end; i++ {
		c := m.chats[i]
		mark := " "
		if c.Unread > 0 {
			mark = "*"
		}
		line := fmt.Sprintf("%s [%-10s] %4d  %s", mark, truncate(c.Network, 10), c.Unread, c.Title)
		if i == m.selected {
			line = sel.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(m.statusBar())
	return b.String()
}

func (m Model) statusBar() string {
	return fmt.Sprintf("NORMAL  %d chats · j/k move · enter open · q quit", len(m.chats))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
```

**IMPORTANT for the implementer:** verify `lipgloss.NewStyle().Bold(true).Render(string)` against Task 1's `go doc`. If method names differ, adjust. Bold is terminal-agnostic and safe for the selection highlight.

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: list render tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): render chat list"
```

---

## Task 12 — View: conversation

**Files:** Modify `internal/ui/view.go`, `internal/ui/view_test.go`

- [ ] **Step 1: Add failing tests**

Append to `internal/ui/view_test.go`:

```go
func TestRender_ConversationShowsMessages(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		selected:      0,
		messages: []api.Message{
			{ID: "m1", SenderName: "Alice", Text: "hey there", IsFromMe: false},
			{ID: "m2", SenderName: "Me", Text: "hi back", IsFromMe: true},
		},
	}
	out := m.render()
	if !strings.Contains(out, "hey there") || !strings.Contains(out, "hi back") {
		t.Errorf("conversation missing message text: %q", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("conversation missing chat title/sender: %q", out)
	}
}

func TestRender_ConversationLoading(t *testing.T) {
	m := Model{mode: ModeConversation, loadingMsgs: true, width: 80, height: 24,
		chats: []api.Chat{{ID: "a", Title: "Alice"}}, currentChatID: "a"}
	if !strings.Contains(m.render(), "Loading") {
		t.Errorf("expected loading text: %q", m.render())
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestRender_Conversation`
Expected: FAIL — `renderConversation` undefined (referenced in `render()` from Task 11; if Task 11 left it referenced, this is a build failure until implemented now).

- [ ] **Step 3: Implement**

Append to `internal/ui/view.go`:

```go
func (m Model) chatTitle(id string) string {
	for _, c := range m.chats {
		if c.ID == id {
			return c.Title
		}
	}
	return id
}

func (m Model) renderConversation() string {
	var b strings.Builder
	b.WriteString(m.chatTitle(m.currentChatID) + "\n")
	if m.loadingMsgs {
		b.WriteString("Loading messages…\n")
		b.WriteString(m.convStatusBar())
		return b.String()
	}
	vr := m.visibleRows()
	end := m.msgOffset + vr
	if end > len(m.messages) {
		end = len(m.messages)
	}
	for i := m.msgOffset; i < end; i++ {
		msg := m.messages[i]
		who := msg.SenderName
		if msg.IsFromMe {
			who = "You"
		}
		ts := msg.Timestamp.Format("15:04")
		b.WriteString(fmt.Sprintf("%s  %-12s  %s\n", ts, truncate(who, 12), msg.Text))
	}
	b.WriteString(m.convStatusBar())
	return b.String()
}

func (m Model) convStatusBar() string {
	return "NORMAL  j/k scroll · esc back · q quit"
}
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: all render tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): render conversation messages"
```

---

## Task 13 — Full ui suite green + go vet

**Files:** none (verification task)

- [ ] **Step 1: Run everything**

Run: `go test ./... && go vet ./...`
Expected: all packages (config, state, api, ui) pass; vet clean.

- [ ] **Step 2: If anything fails, fix it** before proceeding. Commit any fix with `fix(ui): …`.

---

## Task 14 — Wire the TUI into main.go

**Files:** Modify `cmd/beeper-tui/main.go`

- [ ] **Step 1: Replace main.go**

```go
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "No BEEPER_ACCESS_TOKEN set. Enable the Desktop API in Beeper (Settings -> Developers -> Approved connections) and export a token.")
		os.Exit(1)
	}
	client := api.New(cfg)
	if _, err := tea.NewProgram(ui.New(client), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
}
```

**IMPORTANT for the implementer:** confirm `tea.WithAltScreen()` exists in v2. If v2 moved alt-screen onto the `tea.View` struct instead, remove `tea.WithAltScreen()` and set it in `View()`: `v := tea.NewView(m.render()); v.AltScreen = true; return v`. Use whichever Task 1 confirmed. Pick exactly one alt-screen mechanism.

- [ ] **Step 2: Build**

Run: `go build ./cmd/beeper-tui`
Expected: clean.

- [ ] **Step 3: HANDED TO USER for live verification (data rule).**

Do NOT run `./beeper-tui` yourself (it would print real chats/messages into the conversation). Instead, report to the controller that the user should run, in fish, with Beeper Desktop open:
```
fish -c './beeper-tui'
```
and confirm: loading → chat list → j/k moves selection → Enter opens a chat → messages show → j/k scrolls → Esc returns → q quits. The user reports pass/fail.

- [ ] **Step 4: Verify the no-token path (safe, no real data)**

Run: `env -u BEEPER_ACCESS_TOKEN ./beeper-tui`
Expected: prints the "No BEEPER_ACCESS_TOKEN" guidance, exits non-zero, no panic.

- [ ] **Step 5: Commit**

```bash
git add cmd/beeper-tui/main.go
git commit -m "feat(main): launch read-mode bubbletea TUI"
```

---

## Task 15 — Integration test (build-tagged, counts only)

**Files:** Create `internal/ui/integration_test.go`

- [ ] **Step 1: Write it**

```go
//go:build integration

package ui

import (
	"context"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

// go test -tags=integration ./internal/ui/...
// Requires Beeper Desktop running + BEEPER_ACCESS_TOKEN. Asserts COUNTS only —
// never prints chat/message content (data rule).
func TestIntegration_LoadChatsThenMessages(t *testing.T) {
	cfg, err := config.Load()
	if err != nil || cfg.Token == "" {
		t.Skip("no token / config; skipping integration test")
	}
	client := api.New(cfg)
	chats, err := client.ListChats(context.Background())
	if err != nil {
		t.Fatalf("ListChats: %v", err)
	}
	if len(chats) == 0 {
		t.Fatal("expected at least one chat")
	}
	msgs, err := client.ListMessages(context.Background(), chats[0].ID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	t.Logf("loaded %d chats; first chat has %d messages", len(chats), len(msgs))
}
```

- [ ] **Step 2: Confirm excluded from normal suite**

Run: `go test ./internal/ui/...`
Expected: integration test does NOT run.

- [ ] **Step 3: HANDED TO USER** — the user may run `fish -c 'go test -tags=integration ./internal/ui/...'` themselves; it logs only counts, no content. Report pass/fail.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/integration_test.go
git commit -m "test(ui): build-tagged integration test (counts only)"
```

---

## Task 16 — Re-scope bd issues + tag

- [ ] **Step 1: Close the superseded/now-done UI issues**

```bash
bd close beeper-tui-qbg.6 --reason "M1: Model + Mode machine + reducer + async loads implemented (read mode)."
bd close beeper-tui-qbg.7 --reason "M1: List + conversation views (lipgloss v2). Two-pane triage superseded by read→reply full-width design."
bd close beeper-tui-qbg.8 --reason "M1: j/k/gg/G navigation + enter-open + esc-back implemented for read mode."
```

- [ ] **Step 2: Tag**

```bash
go test ./... && go vet ./... && git tag -a milestone-1-read -m "M1: read — launch, browse chats, open a conversation, read + scroll messages, mark-read"
```

- [ ] **Step 3: Note next** — M1.5 (fuzzy `<leader>ff`, hotlist bar, `<leader>l` toggle, `]u`/`[u`) and M2 (reply/INSERT) remain. `bd ready` to see remaining.

---

## Self-Review notes (for the controller)

- **bubbletea v2 risk** mitigated by the Task 1 spike (captures `tea.NewView`, key/window msgs, lipgloss methods, alt-screen mechanism). Tasks 11/14 carry "verify against Task 1" notes for the spots most likely to differ (lipgloss style methods, `WithAltScreen` vs `View.AltScreen`).
- **Testability:** all nav/state logic is pure methods on `Model` (cursorDown/Up, jumpTop/Bottom, openSelected, backToList, clampWindow), tested directly. Only Task 10 constructs a real `tea.WindowSizeMsg` (a plain struct — safe).
- **Spec coverage:** launch+loading (Task 3/11), list nav (5,6), open+mark-read (7), conversation render+scroll (9,12), esc back (8), quit (5), resize (10), wire main (14). Mode machine (2) is built so M2 INSERT + M1.5 overlays slot in.
- **Deferred, flagged:** fuzzy `<leader>ff`, hotlist bar, `<leader>l` toggle, `]u`/`[u` → M1.5. Reply/INSERT → M2. The `:` command-line is M1.5 (M1 quits via bare `q`/`ctrl+c`).
- **Data rule honored:** every test uses synthetic data; live runs (Tasks 14, 15) are explicitly handed to the user, never executed by the implementer, so no real chats/messages enter the conversation.
- **No placeholders** in logic; the few `tea.Cmd`/`tea.Quit`/lipgloss specifics flagged for `go doc` verification are honest (v2 is new) and backstopped by the spike + the user's live smoke test.
