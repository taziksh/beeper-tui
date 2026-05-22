# Phase 3 — Interactive TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the current print-and-exit binary into an interactive bubbletea TUI: launch `beeper-tui`, see a scrollable two-pane screen (chat list + a basic preview of the selected chat), navigate with `j`/`k`/arrows/`gg`/`G`, and quit with `q`.

**Architecture:** A new `internal/ui` package built on bubbletea v2's Elm architecture (`Model`/`Init`/`Update`/`View`). The chat list loads asynchronously via a `tea.Cmd` that calls Phase 2's `api.Client.ListChats`, so the UI never blocks. Navigation logic lives in **pure methods on `Model`** (`moveDown`, `moveUp`, `jumpTop`, `jumpBottom`) that are unit-tested directly — `Update` is a thin layer mapping keys to those methods. The list is windowed (render only the visible slice) so 900+ chats stay snappy. `main.go` shrinks to: load config, construct the UI model, run the program.

**Tech Stack:**
- Go 1.26.3
- bubbletea v2: `charm.land/bubbletea/v2` (NOT the old `github.com/charmbracelet/bubbletea` — v2 shipped Feb 2026 with breaking changes)
- lipgloss v2: `charm.land/lipgloss/v2` (styling)
- Phase 2's `internal/api` (Client.ListChats), Phase 1's `internal/config`

**bd issues:** `beeper-tui-qbg.6` (U1: Model+reducer), `beeper-tui-qbg.7` (U2: View), `beeper-tui-qbg.8` (X1: navigation).

**Deliberately deferred to Phase 4** (do NOT build these here): account filter overlay (`a`), debounced message-history preview via `ListMessages`, full-screen reading mode (`enter`), confirm-on-quit / help / search overlays, new-message handling (needs WebSocket). **Quit is plain `q`** in Phase 3 — the confirm-on-quit overlay from the spec lands in Phase 4 with the other overlays. This is a deliberate, temporary simplification.

**Module path:** `github.com/taziksh/beeper-tui`

---

## Verified bubbletea v2 API (captured 2026-05-20)

```go
import tea "charm.land/bubbletea/v2"

// Model interface:
func (m model) Init() tea.Cmd
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m model) View() tea.View   // NOT string

// View construction:
return tea.NewView("some string")
// or for full-screen:
var v tea.View; v.SetContent("..."); v.AltScreen = true; return v

// Keys:
case tea.KeyPressMsg:
    switch msg.String() { case "ctrl+c", "q": return m, tea.Quit; case "j", "down": ... }

// Program:
p := tea.NewProgram(initialModel)
if _, err := p.Run(); err != nil { ... }

// Commands are funcs returning a msg:
func loadChats() tea.Msg { /* do work */ return chatsLoadedMsg{...} }
```

Items still to confirm during Task 1 spike: exact `tea.WindowSizeMsg` field names (`Width`/`Height`), and lipgloss v2 styling API (`lipgloss.NewStyle()...`). The spike captures these before later tasks rely on them.

---

## File Structure

| File | Task | Responsibility |
|---|---|---|
| `go.mod`/`go.sum` | 1 | Gain bubbletea v2 + lipgloss v2 deps |
| `internal/ui/spike_test.go` (temp) | 1 | Throwaway: prove a v2 program compiles/runs; deleted in Task 1 |
| `internal/ui/model.go` | 2 | `Model` struct, `New(api.Client)`, `Init()` |
| `internal/ui/messages.go` | 3 | custom msgs (`chatsLoadedMsg`, `errMsg`) + `loadChatsCmd` |
| `internal/ui/nav.go` | 4,5,6 | pure navigation methods (`moveDown`/`moveUp`/`jumpTop`/`jumpBottom`/`clampWindow`) |
| `internal/ui/update.go` | 3,4,7 | `Update` — maps msgs/keys to model changes |
| `internal/ui/view.go` | 8,9,10 | `View` + render helpers (list, preview, status bar, loading/error) |
| `internal/ui/nav_test.go` | 4,5,6 | pure unit tests for navigation |
| `internal/ui/update_test.go` | 3 | reducer tests for msg handling |
| `internal/ui/view_test.go` | 8,9,10 | render substring tests |
| `cmd/beeper-tui/main.go` | 11 | shrink to: load config, run the UI program |

---

## Conventions

- Conventional Commits, one commit per task.
- Navigation/logic lives in pure methods returning a new `Model` (value receiver, returns `Model`) so tests never construct `tea.Msg` values.
- `Update` stays thin: translate `tea.KeyPressMsg.String()` → a pure method call.
- Test names `Test<Method>_<Scenario>`.

---

## Task 1 — Spike: add bubbletea v2 + lipgloss v2, prove a program runs

**Files:**
- Modify: `go.mod`, `go.sum`
- Create (temporary): `internal/ui/spike/main.go`

- [ ] **Step 1: Add dependencies**

Run:
```bash
cd /Users/tazik/Projects/beeper-tui
go get charm.land/bubbletea/v2@latest
go get charm.land/lipgloss/v2@latest
```
Expected: go.mod gains both requires; go.sum populated.

- [ ] **Step 2: Write a minimal v2 program**

Create `internal/ui/spike/main.go`:

```go
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

type model struct {
	width, height int
	lastKey       string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		m.lastKey = msg.String()
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(fmt.Sprintf("size=%dx%d lastKey=%q\npress q to quit\n", m.width, m.height, m.lastKey))
}

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build it**

Run: `go build ./internal/ui/spike`
Expected: clean build. If `tea.WindowSizeMsg`, `.Width`/`.Height`, `tea.KeyPressMsg`, `.String()`, `tea.NewView`, or `tea.NewProgram(...).Run()` don't compile, run `go doc charm.land/bubbletea/v2.WindowSizeMsg`, `go doc charm.land/bubbletea/v2.KeyPressMsg`, `go doc charm.land/bubbletea/v2.View`, and adjust. **Report the exact confirmed signatures** — later tasks depend on them.

- [ ] **Step 4: Capture the lipgloss v2 styling API**

Run: `go doc charm.land/lipgloss/v2.NewStyle` and `go doc charm.land/lipgloss/v2.Style` (list its main methods like Foreground, Background, Bold, Width, Padding, Render). Also run `go doc charm.land/lipgloss/v2.JoinHorizontal`. **Paste the output in your report** — the View tasks need the exact styling method names.

- [ ] **Step 5: Delete the spike, commit the deps**

```bash
rm -rf internal/ui/spike
git add go.mod go.sum
git commit -m "chore(ui): add bubbletea v2 + lipgloss v2 dependencies"
```

- [ ] **Step 6: Report**

Report the confirmed: WindowSizeMsg field names, KeyPressMsg.String() behavior, View construction, and the lipgloss v2 Style method names + JoinHorizontal signature. These feed Tasks 2-10.

---

## Task 2 — Model struct + constructor

**Files:**
- Create: `internal/ui/model.go`

- [ ] **Step 1: Write the model**

Create `internal/ui/model.go`:

```go
package ui

import (
	"github.com/taziksh/beeper-tui/internal/api"
)

// Model holds all TUI state. Value receiver everywhere — bubbletea passes it
// by value through Update.
type Model struct {
	client *api.Client

	chats    []api.Chat
	selected int // index into chats
	offset   int // index of the first visible row (windowing)

	width  int
	height int

	loading bool
	err     error
}

// New builds the initial model. The chat fetch happens in Init, not here.
func New(client *api.Client) Model {
	return Model{
		client:  client,
		loading: true,
	}
}
```

- [ ] **Step 2: Build**

Run: `go build ./internal/ui/...`
Expected: clean (unused fields are fine; Go only errors on unused imports/locals, not struct fields).

- [ ] **Step 3: Commit**

```bash
git add internal/ui/model.go
git commit -m "feat(ui): add Model struct and constructor"
```

---

## Task 3 — Async chat loading (Init + command + reducer)

**Files:**
- Create: `internal/ui/messages.go`
- Create: `internal/ui/update.go`
- Create: `internal/ui/update_test.go`

- [ ] **Step 1: Write the failing reducer test**

Create `internal/ui/update_test.go`:

```go
package ui

import (
	"errors"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestUpdate_ChatsLoadedPopulatesModel(t *testing.T) {
	m := Model{loading: true}
	chats := []api.Chat{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}

	updated, _ := m.Update(chatsLoadedMsg{chats: chats})
	got := updated.(Model)

	if got.loading {
		t.Error("loading should be false after chats load")
	}
	if len(got.chats) != 2 {
		t.Fatalf("got %d chats, want 2", len(got.chats))
	}
	if got.chats[0].Title != "A" {
		t.Errorf("chats[0].Title = %q, want A", got.chats[0].Title)
	}
}

func TestUpdate_ErrMsgSetsError(t *testing.T) {
	m := Model{loading: true}
	updated, _ := m.Update(errMsg{err: errors.New("boom")})
	got := updated.(Model)
	if got.loading {
		t.Error("loading should be false after error")
	}
	if got.err == nil {
		t.Error("err should be set")
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/...`
Expected: build failure — `chatsLoadedMsg`, `errMsg`, and `Update` undefined.

- [ ] **Step 3: Write messages + command**

Create `internal/ui/messages.go`:

```go
package ui

import (
	"context"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

type chatsLoadedMsg struct{ chats []api.Chat }
type errMsg struct{ err error }

// loadChatsCmd fetches chats off the UI thread and returns a message.
func (m Model) loadChatsCmd() func() tea_Msg {
	client := m.client
	return func() tea_Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		chats, err := client.ListChats(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return chatsLoadedMsg{chats: chats}
	}
}
```

**IMPORTANT for the implementer:** The return type is wrong above — `tea_Msg` is a placeholder to avoid importing tea in this snippet. Replace `func() tea_Msg` with `tea.Cmd` and import `tea "charm.land/bubbletea/v2"`. A `tea.Cmd` IS `func() tea.Msg`, so the function body returning `chatsLoadedMsg`/`errMsg` is correct. Final signature: `func (m Model) loadChatsCmd() tea.Cmd`.

- [ ] **Step 4: Write Init + Update skeleton**

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
		m.loading = false
		return m, nil
	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}
```

(Use the exact `tea.WindowSizeMsg` field names confirmed in Task 1; `Width`/`Height` are expected.)

- [ ] **Step 5: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: both tests PASS. `go build ./...` clean.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/messages.go internal/ui/update.go internal/ui/update_test.go
git commit -m "feat(ui): async chat loading with Init/Update reducer"
```

---

## Task 4 — Navigation: move down/up with clamping

**Files:**
- Create: `internal/ui/nav.go`
- Create: `internal/ui/nav_test.go`
- Modify: `internal/ui/update.go`

- [ ] **Step 1: Write failing tests**

Create `internal/ui/nav_test.go`:

```go
package ui

import (
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func threeChats() []api.Chat {
	return []api.Chat{{ID: "a"}, {ID: "b"}, {ID: "c"}}
}

func TestMoveDown_AdvancesSelection(t *testing.T) {
	m := Model{chats: threeChats(), selected: 0, height: 10}
	m = m.moveDown()
	if m.selected != 1 {
		t.Errorf("selected = %d, want 1", m.selected)
	}
}

func TestMoveDown_ClampsAtBottom(t *testing.T) {
	m := Model{chats: threeChats(), selected: 2, height: 10}
	m = m.moveDown()
	if m.selected != 2 {
		t.Errorf("selected = %d, want clamped at 2", m.selected)
	}
}

func TestMoveUp_ClampsAtTop(t *testing.T) {
	m := Model{chats: threeChats(), selected: 0, height: 10}
	m = m.moveUp()
	if m.selected != 0 {
		t.Errorf("selected = %d, want clamped at 0", m.selected)
	}
}

func TestMoveDown_EmptyListNoPanic(t *testing.T) {
	m := Model{chats: nil, selected: 0, height: 10}
	m = m.moveDown() // must not panic or go negative
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0 on empty list", m.selected)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestMove`
Expected: build failure — `m.moveDown`/`m.moveUp` undefined.

- [ ] **Step 3: Implement nav**

Create `internal/ui/nav.go`:

```go
package ui

// visibleRows returns how many chat rows fit in the list pane. It reserves
// rows for the status bar (1) and a header (1). Falls back to a sane minimum
// before the first WindowSizeMsg sets height.
func (m Model) visibleRows() int {
	rows := m.height - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func (m Model) moveDown() Model {
	if len(m.chats) == 0 {
		return m
	}
	if m.selected < len(m.chats)-1 {
		m.selected++
	}
	return m.clampWindow()
}

func (m Model) moveUp() Model {
	if len(m.chats) == 0 {
		return m
	}
	if m.selected > 0 {
		m.selected--
	}
	return m.clampWindow()
}

// clampWindow keeps `selected` visible within the [offset, offset+visibleRows)
// window, scrolling offset as needed.
func (m Model) clampWindow() Model {
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
	return m
}
```

- [ ] **Step 4: Wire keys in Update**

In `internal/ui/update.go`, extend the `tea.KeyPressMsg` switch:

```go
		case "j", "down":
			return m.moveDown(), nil
		case "k", "up":
			return m.moveUp(), nil
```

(Place these alongside the existing `"ctrl+c", "q"` case.)

- [ ] **Step 5: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: all nav tests + prior reducer tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/nav.go internal/ui/nav_test.go internal/ui/update.go
git commit -m "feat(ui): j/k navigation with clamping and windowing"
```

---

## Task 5 — Navigation: gg/G jump to top/bottom

**Files:**
- Modify: `internal/ui/nav.go`
- Modify: `internal/ui/nav_test.go`
- Modify: `internal/ui/update.go`

- [ ] **Step 1: Add failing tests**

Append to `internal/ui/nav_test.go`:

```go
func TestJumpBottom_SelectsLast(t *testing.T) {
	m := Model{chats: threeChats(), selected: 0, height: 10}
	m = m.jumpBottom()
	if m.selected != 2 {
		t.Errorf("selected = %d, want 2", m.selected)
	}
}

func TestJumpTop_SelectsFirst(t *testing.T) {
	m := Model{chats: threeChats(), selected: 2, offset: 2, height: 10}
	m = m.jumpTop()
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0", m.selected)
	}
	if m.offset != 0 {
		t.Errorf("offset = %d, want 0", m.offset)
	}
}

func TestJumpBottom_EmptyListNoPanic(t *testing.T) {
	m := Model{chats: nil}
	m = m.jumpBottom()
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0", m.selected)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestJump`
Expected: `m.jumpTop`/`m.jumpBottom` undefined.

- [ ] **Step 3: Implement**

Append to `internal/ui/nav.go`:

```go
func (m Model) jumpTop() Model {
	m.selected = 0
	return m.clampWindow()
}

func (m Model) jumpBottom() Model {
	if len(m.chats) == 0 {
		m.selected = 0
		return m
	}
	m.selected = len(m.chats) - 1
	return m.clampWindow()
}
```

- [ ] **Step 4: Wire keys**

In `internal/ui/update.go` add to the key switch. `G` jumps bottom. For `gg` (two presses of `g`), track a pending-`g` flag on the model:

In `model.go`, add a field to the struct: `pendingG bool`.

In the key switch:

```go
		case "G":
			m.pendingG = false
			return m.jumpBottom(), nil
		case "g":
			if m.pendingG {
				m.pendingG = false
				return m.jumpTop(), nil
			}
			m.pendingG = true
			return m, nil
```

And at the TOP of the `tea.KeyPressMsg` case (before the inner switch), reset the flag for any non-`g` key so a stray `g` doesn't linger:

```go
	case tea.KeyPressMsg:
		key := msg.String()
		if key != "g" {
			m.pendingG = false
		}
		switch key {
		// ...existing cases, but use `key` instead of msg.String()...
		}
```

(Refactor the existing cases to switch on `key`.)

- [ ] **Step 5: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: jump tests pass; prior tests still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/nav.go internal/ui/nav_test.go internal/ui/update.go internal/ui/model.go
git commit -m "feat(ui): gg/G jump to top/bottom"
```

---

## Task 6 — Windowing test (scroll offset with a realistic viewport)

**Files:**
- Modify: `internal/ui/nav_test.go`

- [ ] **Step 1: Add the test**

Append to `internal/ui/nav_test.go`:

```go
func manyChats(n int) []api.Chat {
	cs := make([]api.Chat, n)
	for i := range cs {
		cs[i] = api.Chat{ID: string(rune('a' + i%26))}
	}
	return cs
}

func TestWindow_OffsetFollowsSelectionDown(t *testing.T) {
	// height 7 -> visibleRows = 5. Move down 6 times from top.
	m := Model{chats: manyChats(20), selected: 0, offset: 0, height: 7}
	for i := 0; i < 6; i++ {
		m = m.moveDown()
	}
	if m.selected != 6 {
		t.Fatalf("selected = %d, want 6", m.selected)
	}
	// selected 6 must be visible: offset <= 6 < offset+5
	if !(m.offset <= 6 && 6 < m.offset+5) {
		t.Errorf("offset = %d does not keep selected=6 visible (vr=5)", m.offset)
	}
}

func TestWindow_OffsetFollowsSelectionUp(t *testing.T) {
	m := Model{chats: manyChats(20), selected: 10, offset: 6, height: 7}
	for i := 0; i < 8; i++ {
		m = m.moveUp()
	}
	if m.selected != 2 {
		t.Fatalf("selected = %d, want 2", m.selected)
	}
	if m.offset > m.selected {
		t.Errorf("offset = %d should be <= selected = %d", m.offset, m.selected)
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/ui/... -run TestWindow -v`
Expected: PASS (the windowing logic from Task 4's `clampWindow` should already satisfy these). If they fail, fix `clampWindow` in `nav.go` until they pass.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/nav_test.go
git commit -m "test(ui): cover window-offset scrolling behavior"
```

---

## Task 7 — WindowSizeMsg re-clamps the window

**Files:**
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/update_test.go`

- [ ] **Step 1: Add failing test**

Append to `internal/ui/update_test.go`:

```go
import (
	// add to existing imports:
	tea "charm.land/bubbletea/v2"
)

func TestUpdate_WindowSizeReclampsOffset(t *testing.T) {
	// selected far down with a big window, then the terminal shrinks.
	m := Model{chats: manyChats(50), selected: 40, offset: 36, height: 10, width: 80}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 6})
	got := updated.(Model)
	if got.height != 6 {
		t.Errorf("height = %d, want 6", got.height)
	}
	// vr = 4 now; selected 40 must remain visible
	if !(got.offset <= 40 && 40 < got.offset+got.visibleRows()) {
		t.Errorf("offset = %d does not keep selected visible after resize", got.offset)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestUpdate_WindowSize`
Expected: FAIL — current `WindowSizeMsg` handler sets height but doesn't re-clamp.

- [ ] **Step 3: Fix the handler**

In `internal/ui/update.go`, update the `tea.WindowSizeMsg` case:

```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.clampWindow(), nil
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/update.go internal/ui/update_test.go
git commit -m "feat(ui): re-clamp window on terminal resize"
```

---

## Task 8 — View: chat list pane

**Files:**
- Create: `internal/ui/view.go`
- Create: `internal/ui/view_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ui/view_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestView_LoadingState(t *testing.T) {
	m := Model{loading: true, width: 80, height: 24}
	out := m.View().String() // see note: tea.View stringifies its content
	if !strings.Contains(out, "Loading") {
		t.Errorf("loading view missing 'Loading': %q", out)
	}
}

func TestView_ListShowsChatTitles(t *testing.T) {
	m := Model{
		width: 80, height: 24,
		chats: []api.Chat{
			{ID: "a", Network: "Signal", Title: "Bob", Unread: 0},
			{ID: "b", Network: "WhatsApp", Title: "Work Chat", Unread: 140},
		},
		selected: 0,
	}
	out := m.View().String()
	if !strings.Contains(out, "Bob") {
		t.Errorf("view missing chat title 'Bob': %q", out)
	}
	if !strings.Contains(out, "Work Chat") {
		t.Errorf("view missing chat title 'LPM': %q", out)
	}
	if !strings.Contains(out, "140") {
		t.Errorf("view missing unread count '140': %q", out)
	}
}
```

**IMPORTANT for the implementer:** `tea.View` may not have a `.String()` method. In Task 1 you confirmed how `tea.View` exposes its content. If `tea.View` has no `String()`, change the View tasks to test a helper `m.render() string` that produces the content, and have `View()` wrap it: `func (m Model) View() tea.View { return tea.NewView(m.render()) }`. Test `m.render()` directly. Use whichever the confirmed v2 API supports — adjust these tests to call `m.render()` if `tea.View.String()` doesn't exist. Prefer the `render()` helper approach since it's cleanly testable regardless.

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestView`
Expected: build failure — `View`/`render` undefined.

- [ ] **Step 3: Implement the list rendering**

Create `internal/ui/view.go`. Use the lipgloss v2 styling API confirmed in Task 1. This reference implementation uses a `render()` helper (testable) that `View()` wraps:

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
	if m.loading {
		return "Loading chats…\n"
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	return m.renderList()
}

func (m Model) renderList() string {
	selStyle := lipgloss.NewStyle().Bold(true)

	var b strings.Builder
	vr := m.visibleRows()
	end := m.offset + vr
	if end > len(m.chats) {
		end = len(m.chats)
	}
	for i := m.offset; i < end; i++ {
		c := m.chats[i]
		marker := " "
		if c.Unread > 0 {
			marker = "*"
		}
		line := fmt.Sprintf("%s [%-10s] %4d  %s", marker, truncate(c.Network, 10), c.Unread, c.Title)
		if i == m.selected {
			line = selStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
```

**IMPORTANT for the implementer:** Confirm `lipgloss.NewStyle().Bold(true).Render(string)` is the v2 API (it should be, per Task 1's `go doc`). If method names differ, adjust. The selection highlight can be Bold or a background color — Bold is safe and terminal-agnostic.

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: View tests pass. If you switched the tests to `m.render()`, ensure they call that.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): render scrollable chat list pane"
```

---

## Task 9 — View: preview pane (two-pane layout)

**Files:**
- Modify: `internal/ui/view.go`
- Modify: `internal/ui/view_test.go`

- [ ] **Step 1: Add failing test**

Append to `internal/ui/view_test.go`:

```go
func TestView_PreviewShowsSelectedChat(t *testing.T) {
	m := Model{
		width: 100, height: 24,
		chats: []api.Chat{
			{ID: "a", Network: "WhatsApp", Title: "Trip Planning", Unread: 944, Preview: "sounds good"},
		},
		selected: 0,
	}
	out := m.render()
	if !strings.Contains(out, "Trip Planning") {
		t.Errorf("preview missing selected title: %q", out)
	}
	if !strings.Contains(out, "sounds good") {
		t.Errorf("preview missing last-message text: %q", out)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestView_Preview`
Expected: FAIL — preview text not in output yet (render only shows the list).

- [ ] **Step 3: Implement preview + horizontal join**

In `internal/ui/view.go`, add a preview renderer and join it beside the list. Replace `renderList`'s use in `render()` with a two-pane composition:

```go
func (m Model) render() string {
	if m.loading {
		return "Loading chats…\n"
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	listW := m.width / 2
	if listW < 20 {
		listW = m.width // narrow terminal: list only
	}
	list := lipgloss.NewStyle().Width(listW).Render(m.renderList())
	if listW == m.width {
		return list
	}
	preview := lipgloss.NewStyle().Width(m.width - listW).Render(m.renderPreview())
	return lipgloss.JoinHorizontal(lipgloss.Top, list, preview)
}

func (m Model) renderPreview() string {
	if len(m.chats) == 0 || m.selected >= len(m.chats) {
		return "No chat selected"
	}
	c := m.chats[m.selected]
	var b strings.Builder
	b.WriteString(c.Title)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s · %d unread\n", c.Network, c.Unread))
	b.WriteString("\n")
	if c.Preview != "" {
		b.WriteString(c.Preview)
	} else {
		b.WriteString("(no preview)")
	}
	b.WriteString("\n")
	return b.String()
}
```

**IMPORTANT for the implementer:** Confirm `lipgloss.JoinHorizontal(lipgloss.Top, ...)` and `lipgloss.NewStyle().Width(n).Render(s)` against Task 1's `go doc` output. `lipgloss.Top` is a position constant — verify its exact name (might be `lipgloss.Top` or under a `Position` type). Adjust if needed.

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: all View tests pass, including the preview.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): two-pane layout with selected-chat preview"
```

---

## Task 10 — View: status bar + chat count header

**Files:**
- Modify: `internal/ui/view.go`
- Modify: `internal/ui/view_test.go`

- [ ] **Step 1: Add failing test**

Append to `internal/ui/view_test.go`:

```go
func TestView_StatusBarShowsHintsAndCount(t *testing.T) {
	m := Model{
		width: 100, height: 24,
		chats:    []api.Chat{{ID: "a", Title: "A"}},
		selected: 0,
	}
	out := m.render()
	if !strings.Contains(out, "j/k") {
		t.Errorf("status bar missing nav hint 'j/k': %q", out)
	}
	if !strings.Contains(out, "q") {
		t.Errorf("status bar missing quit hint 'q': %q", out)
	}
	if !strings.Contains(out, "1 chats") {
		t.Errorf("status bar missing chat count '1 chats': %q", out)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/ui/... -run TestView_StatusBar`
Expected: FAIL — no status bar yet.

- [ ] **Step 3: Implement status bar**

In `internal/ui/view.go`, append a status bar to `render()`'s output. Change the final return paths so the two-pane body has a status line beneath it:

```go
func (m Model) statusBar() string {
	return fmt.Sprintf("%d chats · j/k or ↑/↓ move · gg/G top/bottom · q quit", len(m.chats))
}
```

And update `render()` to append it. For the two-pane branch:

```go
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, preview)
	return body + "\n" + m.statusBar() + "\n"
```

For the narrow (list-only) branch:

```go
	return list + "\n" + m.statusBar() + "\n"
```

(Apply the status bar to both non-loading, non-error return paths.)

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/ui/... -v`
Expected: all View tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): status bar with nav hints and chat count"
```

---

## Task 11 — Wire the TUI into main.go (the interactive payoff)

**Files:**
- Modify: `cmd/beeper-tui/main.go`

- [ ] **Step 1: Replace main.go**

Replace `cmd/beeper-tui/main.go` ENTIRELY with:

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

**IMPORTANT for the implementer:** Confirm `tea.WithAltScreen()` exists in v2 (it enables full-screen mode). If v2 moved alt-screen to the `tea.View` struct (`v.AltScreen = true`) instead of a program option, REMOVE `tea.WithAltScreen()` here and instead set `AltScreen` in the View — in `view.go`, change `View()` to:
```go
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}
```
Use whichever the confirmed Task 1 API supports. Pick ONE alt-screen mechanism, not both.

- [ ] **Step 2: Build**

Run: `go build ./cmd/beeper-tui`
Expected: clean.

- [ ] **Step 3: Manual smoke test (requires Beeper Desktop running + token)**

The token is a fish universal var, so run via fish:
```bash
fish -c './beeper-tui'
```
Expected: full-screen TUI. Briefly shows "Loading chats…", then the two-pane list. Press `j`/`k` to move the selection (the `>` marker and the preview pane update). `gg`/`G` jump. `q` quits cleanly back to the shell.

If Beeper Desktop is not running, the UI will show the error state ("Error: ...") — that's acceptable; the interactivity (q to quit) still works. Report what you observed.

- [ ] **Step 4: Confirm unit suite still passes**

Run: `go test ./...`
Expected: config, state, api, ui all pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/beeper-tui/main.go internal/ui/view.go
git commit -m "feat(main): launch interactive bubbletea TUI"
```

---

## Task 12 — Close bd issues + tag

- [ ] **Step 1: Close U1, U2, X1**

```bash
bd close beeper-tui-qbg.6 --reason "Model + Init/Update reducer + async chat loading + mode/connection state. Pure nav methods unit-tested."
bd close beeper-tui-qbg.7 --reason "Two-pane View (chat list + preview + status bar) via lipgloss v2; loading/error states. Render helper unit-tested."
bd close beeper-tui-qbg.8 --reason "Chat list navigation: j/k + arrows, gg/G, clamping, windowed scrolling for 900+ chats. Wired into Update."
```

- [ ] **Step 2: Sanity + tag**

```bash
go test ./... && go vet ./... && git tag -a phase-3-complete -m "Phase 3: interactive bubbletea v2 TUI — scrollable two-pane chat list with preview and j/k navigation"
```
Expected: tests pass, vet clean, tag created.

- [ ] **Step 3: Check next work**

```bash
bd ready
```
Expected: Phase 4 candidates surface (X2 debounced preview, X3 reading mode, X4 account filter, X7 overlays).

---

## Self-Review notes (for the controller)

- **bubbletea v2 API risk** is the main hazard, mitigated by the Task 1 spike capturing exact signatures (WindowSizeMsg fields, tea.View construction, lipgloss v2 methods, alt-screen mechanism). Tasks 8/9/11 carry explicit "verify against Task 1" notes for the spots I'm least certain about (tea.View.String() vs a render() helper, JoinHorizontal/position constants, WithAltScreen vs View.AltScreen).
- **Testability:** navigation and rendering logic live in pure methods (`moveDown`, `clampWindow`, `render`) tested without constructing bubbletea internals. Only Task 7 constructs a real `tea.WindowSizeMsg`, which is a plain struct — safe.
- **Spec coverage:** U1 (model+reducer+async load) Tasks 2,3,7; U2 (two-pane view+status) Tasks 8,9,10; X1 (j/k/arrows/gg/G/windowing) Tasks 4,5,6. Loading state covered (Task 8). Quit = plain `q` (Task 3) — confirm-on-quit deferred to Phase 4 as flagged.
- **Deferred, not forgotten:** account filter, debounced message preview, reading mode, overlays, new-message/WS — all Phase 4+. The preview pane here uses the `Preview` text already on `api.Chat` (no extra fetch), so it's instant and needs no debouncing yet.
- **No placeholders** in logic, but several View-layer specifics depend on Task 1's `go doc` capture — honest given v2 is 3 months old and I can't compile against it while planning. The manual smoke test (Task 11) is the backstop that proves it actually renders and navigates.
