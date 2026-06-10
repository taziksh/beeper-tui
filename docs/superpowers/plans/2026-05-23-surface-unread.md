# Surface Unread Messages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make unread chats and messages obvious at a glance — colored + floated to the top of the list, with new messages marked and auto-scrolled-to inside a conversation.

**Architecture:** Pure-function changes to the existing Bubble Tea model. A `sortChats` helper floats unread chats up (keeping selection pinned by chat ID); the `IsUnread` flag is plumbed from the SDK into our `api.Message`; the list and conversation renderers gain an accent style + glyphs; `messagesLoadedMsg` auto-scrolls to the first unread message.

**Tech Stack:** Go, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `github.com/beeper/desktop-api-go/v5`.

---

## File structure

- `internal/api/types.go` — add `Message.IsUnread bool`.
- `internal/api/messages.go` — map `IsUnread` in `mapMessage`.
- `internal/api/messages_test.go` — assert `IsUnread` is carried through.
- `internal/ui/unread.go` *(new)* — `sortChats` + `reselectByID` helpers and the shared accent style / glyph constants. Keeps unread logic in one focused file.
- `internal/ui/unread_test.go` *(new)* — table tests for sort + reselect.
- `internal/ui/update.go` — sort on `chatsLoadedMsg`; first-unread offset on `messagesLoadedMsg`.
- `internal/ui/update_test.go` — auto-scroll-to-first-unread test.
- `internal/ui/view.go` — accent glyph + colored count in `renderList`; `▎` marker in `renderConversation`.
- `internal/ui/view_test.go` — rendering assertions for unread glyph/marker.

A note on testing color: `lipgloss` emits no ANSI escapes under `go test` (no TTY / color profile), so styled text renders as plain runes. **Assert on the glyphs (`●`, `▎`), never on color codes.**

---

## Task 1: Plumb `IsUnread` into `api.Message`

**Files:**
- Modify: `internal/api/types.go:17-25`
- Modify: `internal/api/messages.go:30-39`
- Test: `internal/api/messages_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/api/messages_test.go`:

```go
func TestListMessages_MapsIsUnread(t *testing.T) {
	const json = `{
	  "items": [
	    {"id":"m1","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"1","text":"old","timestamp":"2026-05-19T10:00:00Z","isSender":false,"senderName":"Bob","isUnread":false},
	    {"id":"m2","accountID":"acc","chatID":"chat-1","senderID":"u1","sortKey":"2","text":"new","timestamp":"2026-05-19T10:01:00Z","isSender":false,"senderName":"Bob","isUnread":true}
	  ],
	  "hasMore": false, "oldestCursor": "o", "newestCursor": "n"
	}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(json))
	})

	msgs, err := client.ListMessages(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if msgs[0].IsUnread {
		t.Error("msgs[0].IsUnread = true, want false")
	}
	if !msgs[1].IsUnread {
		t.Error("msgs[1].IsUnread = false, want true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestListMessages_MapsIsUnread`
Expected: FAIL — `msgs[1].IsUnread = false, want true` (field is always false; not yet mapped).

- [ ] **Step 3: Add the field**

In `internal/api/types.go`, add `IsUnread` to the `Message` struct:

```go
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
```

- [ ] **Step 4: Map the field**

In `internal/api/messages.go`, add one line to `mapMessage`:

```go
func mapMessage(m shared.Message) Message {
	return Message{
		ID:         m.ID,
		ChatID:     m.ChatID,
		SenderName: m.SenderName,
		Text:       renderText(m),
		Timestamp:  m.Timestamp,
		IsFromMe:   m.IsSender,
		IsUnread:   m.IsUnread,
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/api/ -run TestListMessages_MapsIsUnread`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/types.go internal/api/messages.go internal/api/messages_test.go
git commit -m "feat(api): plumb per-message IsUnread flag"
```

---

## Task 2: `sortChats` + `reselectByID` helpers

**Files:**
- Create: `internal/ui/unread.go`
- Test: `internal/ui/unread_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ui/unread_test.go`:

```go
package ui

import (
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestSortChats_UnreadFloatToTopThenRecency(t *testing.T) {
	t0 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	chats := []api.Chat{
		{ID: "readOld", Unread: 0, LastActive: t0},
		{ID: "unreadOld", Unread: 2, LastActive: t0},
		{ID: "readNew", Unread: 0, LastActive: t0.Add(2 * time.Hour)},
		{ID: "unreadNew", Unread: 1, LastActive: t0.Add(time.Hour)},
	}
	sortChats(chats)
	got := []string{chats[0].ID, chats[1].ID, chats[2].ID, chats[3].ID}
	want := []string{"unreadNew", "unreadOld", "readNew", "readOld"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortChats order = %v, want %v", got, want)
		}
	}
}

func TestReselectByID_FindsMovedChat(t *testing.T) {
	chats := []api.Chat{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if got := reselectByID(chats, "c"); got != 2 {
		t.Errorf("reselectByID = %d, want 2", got)
	}
}

func TestReselectByID_MissingFallsBackToZero(t *testing.T) {
	chats := []api.Chat{{ID: "a"}, {ID: "b"}}
	if got := reselectByID(chats, "gone"); got != 0 {
		t.Errorf("reselectByID = %d, want 0 (fallback)", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run 'TestSortChats|TestReselectByID'`
Expected: FAIL — `undefined: sortChats`, `undefined: reselectByID`.

- [ ] **Step 3: Write the helpers**

Create `internal/ui/unread.go`:

```go
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

// accentStyle colors unread indicators. ANSI "3" (yellow) respects the user's
// terminal theme; lipgloss renders it as plain text under `go test` (no TTY).
var accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

// sortChats floats unread chats to the top, most-recent-first within each
// group. Stable in spirit: equal keys keep API order via the time comparison.
func sortChats(chats []api.Chat) {
	sort.SliceStable(chats, func(i, j int) bool {
		iUnread, jUnread := chats[i].Unread > 0, chats[j].Unread > 0
		if iUnread != jUnread {
			return iUnread // unread (true) sorts before read (false)
		}
		return chats[i].LastActive.After(chats[j].LastActive)
	})
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/ -run 'TestSortChats|TestReselectByID'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/unread.go internal/ui/unread_test.go
git commit -m "feat(ui): add sortChats and reselectByID unread helpers"
```

---

## Task 3: Sort chats on load (selection pinned by ID)

**Files:**
- Modify: `internal/ui/update.go:13-16`
- Test: `internal/ui/update_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/update_test.go`:

```go
func TestUpdate_ChatsLoaded_SortsUnreadToTopAndPinsSelection(t *testing.T) {
	t0 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	// User had "b" selected before the refresh.
	m := Model{loadingChats: true, chats: []api.Chat{{ID: "a"}, {ID: "b"}}, selected: 1}
	got, _ := m.Update(chatsLoadedMsg{chats: []api.Chat{
		{ID: "a", Unread: 0, LastActive: t0.Add(time.Hour)},
		{ID: "b", Unread: 3, LastActive: t0},
	}})
	gm := got.(Model)
	if gm.chats[0].ID != "b" {
		t.Errorf("chats[0].ID = %q, want b (unread floats up)", gm.chats[0].ID)
	}
	if gm.chats[gm.selected].ID != "b" {
		t.Errorf("selection landed on %q, want b (pinned by ID across re-sort)", gm.chats[gm.selected].ID)
	}
}
```

This test needs `"time"` in the import block — `update_test.go` already imports `errors`, `testing`, `tea`, and `api`; add `"time"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestUpdate_ChatsLoaded_SortsUnreadToTopAndPinsSelection`
Expected: FAIL — `chats[0].ID = "a", want b` (no sort yet).

- [ ] **Step 3: Sort in the handler**

In `internal/ui/update.go`, replace the `chatsLoadedMsg` case:

```go
	case chatsLoadedMsg:
		// Pin the user's selection across the re-sort by chat ID, so unread
		// chats can float to the top without the cursor jumping to a different
		// chat. An empty prior selection (initial load) resolves to index 0.
		var selectedID string
		if m.selected < len(m.chats) {
			selectedID = m.chats[m.selected].ID
		}
		chats := msg.chats
		sortChats(chats)
		m.chats = chats
		if selectedID != "" {
			m.selected = reselectByID(m.chats, selectedID)
		}
		m.loadingChats = false
		return m.clampWindow(), nil
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/ -run TestUpdate_ChatsLoaded`
Expected: PASS (both the new test and the existing `TestUpdate_ChatsLoaded`).

- [ ] **Step 5: Commit**

```bash
git add internal/ui/update.go internal/ui/update_test.go
git commit -m "feat(ui): float unread chats to top, pin selection by ID"
```

---

## Task 4: Accent + glyph in the chat list

**Files:**
- Modify: `internal/ui/view.go:41-54`
- Test: `internal/ui/view_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/view_test.go`:

```go
func TestRender_UnreadChatHasGlyph(t *testing.T) {
	m := Model{
		mode: ModeList, width: 80, height: 24,
		chats: []api.Chat{
			{ID: "a", Network: "Signal", Title: "Alice", Unread: 0},
			{ID: "b", Network: "WhatsApp", Title: "Dev Team", Unread: 5},
		},
		selected: 0,
	}
	out := m.render()
	if !strings.Contains(out, "●") {
		t.Errorf("unread chat row missing ● glyph: %q", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestRender_UnreadChatHasGlyph`
Expected: FAIL — output still uses `*`, no `●`.

- [ ] **Step 3: Update `renderList`**

In `internal/ui/view.go`, replace the row-building loop body inside `renderList` (the `for i := m.offset; i < end; i++` block):

```go
	for i := m.offset; i < end; i++ {
		c := m.chats[i]
		mark := readGlyph
		count := fmt.Sprintf("%4d", c.Unread)
		if c.Unread > 0 {
			mark = accentStyle.Render(unreadGlyph)
			count = accentStyle.Render(count)
		}
		line := fmt.Sprintf("%s [%-10s] %s  %s", mark, truncate(c.Network, 10), count, c.Title)
		if i == m.selected {
			line = sel.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
```

(`sel` is the existing bold style declared at the top of `renderList`; bold stays reserved for selection and composes with the accent.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/ -run 'TestRender_UnreadChatHasGlyph|TestRender_ListShowsTitles'`
Expected: PASS (existing `TestRender_ListShowsTitles` still finds "Dev Team" and "5").

- [ ] **Step 5: Commit**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): accent color and ● glyph for unread chats"
```

---

## Task 5: Auto-scroll to first unread message

**Files:**
- Create helper in: `internal/ui/unread.go`
- Modify: `internal/ui/update.go` (`messagesLoadedMsg` case, lines 17-23)
- Test: `internal/ui/update_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/update_test.go`:

```go
func TestUpdate_MessagesLoaded_ScrollsToFirstUnread(t *testing.T) {
	// height 7 -> visibleRows 5. 10 messages, first unread at index 6.
	ms := make([]api.Message, 10)
	for i := range ms {
		ms[i] = api.Message{ID: fmt.Sprintf("m%d", i), Text: "x"}
	}
	ms[6].IsUnread = true
	ms[7].IsUnread = true
	m := Model{currentChatID: "a", loadingMsgs: true, height: 7}
	got, _ := m.Update(messagesLoadedMsg{chatID: "a", messages: ms})
	gm := got.(Model)
	if gm.msgOffset != 6 {
		t.Errorf("msgOffset = %d, want 6 (first unread at top)", gm.msgOffset)
	}
}
```

This test needs `"fmt"` in `update_test.go`'s import block — add it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestUpdate_MessagesLoaded_ScrollsToFirstUnread`
Expected: FAIL — `msgOffset = 5, want 6` (currently scrolls to bottom = maxMsgOffset).

- [ ] **Step 3: Add the helper**

Append to `internal/ui/unread.go`:

```go
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
```

- [ ] **Step 4: Update the handler**

In `internal/ui/update.go`, replace the `messagesLoadedMsg` case:

```go
	case messagesLoadedMsg:
		if msg.chatID == m.currentChatID {
			m.messages = msg.messages
			m.loadingMsgs = false
			// Land on the first unread message so new content is at the top of
			// the viewport; with nothing unread, fall back to the bottom.
			if u := firstUnreadIndex(m.messages); u >= 0 {
				m.msgOffset = u
				m = m.clampWindow()
			} else {
				m.msgOffset = m.maxMsgOffset()
			}
		}
		return m, nil
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/ui/ -run TestUpdate_MessagesLoaded`
Expected: PASS — the new test plus existing `TestUpdate_MessagesLoadedForCurrentChat` and `TestUpdate_MessagesLoaded_OpensAtBottom` (those use no-unread messages, so the bottom fallback keeps them green).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/unread.go internal/ui/update.go internal/ui/update_test.go
git commit -m "feat(ui): auto-scroll to first unread message on open"
```

---

## Task 6: Per-message unread marker in the conversation view

**Files:**
- Modify: `internal/ui/view.go:90-102`
- Test: `internal/ui/view_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/view_test.go`:

```go
func TestRender_UnreadMessageHasMarker(t *testing.T) {
	m := Model{
		mode: ModeConversation, width: 80, height: 24,
		currentChatID: "a",
		chats:         []api.Chat{{ID: "a", Title: "Alice"}},
		messages: []api.Message{
			{ID: "m1", SenderName: "Alice", Text: "seen this", IsUnread: false},
			{ID: "m2", SenderName: "Alice", Text: "brand new", IsUnread: true},
		},
	}
	out := m.render()
	if !strings.Contains(out, "▎") {
		t.Errorf("unread message row missing ▎ marker: %q", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestRender_UnreadMessageHasMarker`
Expected: FAIL — no `▎` in output.

- [ ] **Step 3: Update `renderConversation`**

In `internal/ui/view.go`, replace the message loop body inside `renderConversation` (the `for i := m.msgOffset; i < end; i++` block):

```go
	for i := m.msgOffset; i < end; i++ {
		msg := m.messages[i]
		who := msg.SenderName
		if msg.IsFromMe {
			who = "You"
		}
		ts := msg.Timestamp.Format("15:04")
		marker := " "
		if msg.IsUnread {
			marker = accentStyle.Render(msgMarker)
		}
		line := fmt.Sprintf("%s %s  %-12s  %s", marker, ts, truncate(who, 12), msg.Text)
		if m.failedSends[msg.ID] {
			line += "  ! send failed"
		}
		b.WriteString(line + "\n")
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/ -run 'TestRender_UnreadMessageHasMarker|TestRender_ConversationShowsMessages|TestRender_FailedSendMarker'`
Expected: PASS (existing conversation render tests still pass — message text, sender, and the failed-send marker are unchanged).

- [ ] **Step 5: Commit**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): accent marker on unread messages in conversation"
```

---

## Task 7: Full verification

- [ ] **Step 1: Run the whole suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 2: Vet and build**

Run: `go vet ./... && go build ./...`
Expected: no output, clean build.

- [ ] **Step 3: Build the binary for the live test**

Run: `go build -o beeper-tui ./cmd/beeper-tui`
Expected: `beeper-tui` binary produced. Per project workflow, hold the final wrap-up until the user live-tests the TUI against their real account, then close `beeper-tui-jfs`.
```

