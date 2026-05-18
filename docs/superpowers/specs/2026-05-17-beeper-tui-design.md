# beeper-tui — v1 design

A keyboard-driven terminal UI for [Beeper](https://beeper.com), built on top of the local Beeper Desktop API. v1 is read-only triage; v2 adds sending; v3 adds rich messaging.

## 1. Goals and scope

### v1 (this spec)
A read-only triage client. From the terminal you can:
- See every chat across every network Beeper covers, sorted by recency
- Filter the chat list to a single account (overlay, on demand)
- Scroll the chat list with j/k or arrow keys
- See a live preview of the highlighted chat in a right pane (debounced, ~150ms dwell)
- Open a chat full-screen for focused reading
- Watch new messages arrive in real time without polling
- Quit cleanly

### v2 (future)
Same UI, plus sending plain-text messages. The WS echo confirms delivery.

### v3 (future)
Attachments (view + send), reactions, replies, threads, edits, deletes.

### Out of scope (probably forever)
- Voice or video calls
- Multi-machine state sync
- Standalone mode without Beeper Desktop running (would require implementing the Matrix protocol and E2EE — a separate project)

### v1 success criteria
- Launch `beeper-tui` → chat list visible in <1s warm / <3s cold
- j/k scrolls the chat list; preview pane updates within ~200ms of selection settling
- New messages in subscribed chats appear in the preview pane within ~500ms of arrival at the Desktop API
- Stays responsive at 200+ chats across 6+ networks
- `q` quits cleanly; subsequent launches feel instant thanks to the on-disk cache
- Previewing a chat does **not** mark it as read on Beeper; only explicit open (enter) does

## 2. Backend choice and connection model

### Why the local Beeper Desktop API (not direct Matrix)
The Beeper Desktop app exposes a local REST + WebSocket API on `http://127.0.0.1:23373` when enabled in Settings → Developers (Beeper Desktop v4.1.169+). Using it means:
- Beeper Desktop handles authentication, end-to-end encryption, bridge wrangling
- We get a stable, documented surface (official Go SDK at `github.com/beeper/desktop-api-go`)
- v1 ships in weeks, not months

The alternative — connecting directly to `matrix.beeper.com` as a Matrix client — is rejected for v1 because implementing Matrix + Olm/Megolm E2EE is a project unto itself. We may revisit for a future "standalone" mode if there's demand.

### Transport: REST for commands and snapshots, WebSocket for events
Beeper Desktop's API has two surfaces and we use both:

- **REST endpoints** (`http://127.0.0.1:23373`) for one-shot operations: list chats, list messages, search, fetch contacts, fetch media. Used for the initial load and for any on-demand fetch.
- **WebSocket** (`/v1/ws`) for live events. After connecting, we send a `subscriptions.set` message declaring which chats we want events for. The stream then delivers new messages, edits, deletions, read receipts, typing indicators, and chat-state changes for those chats only.

This pairing fits the TUI naturally: REST for cold load and user-initiated actions, WS for everything live. We never poll.

### Authentication and first-run UX
1. **Primary path:** the TUI auto-discovers the access token from a locally-running Beeper Desktop (via the Beeper Go SDK's discovery hooks)
2. **Fallback:** `BEEPER_ACCESS_TOKEN` environment variable
3. **If neither works:** show a one-screen onboarding panel explaining the user must enable the Desktop API in Beeper Desktop → Settings → Developers; offer a "retry" key. The TUI does not exit — once the API comes up, it connects.

We never store the token to disk ourselves; Beeper Desktop owns that.

## 3. Architecture

### One process, one event loop, two transport clients

```
        ┌──────────────────────────────────────┐
        │  Beeper Desktop (already running)    │
        │  http://127.0.0.1:23373              │
        └──────────────────────────────────────┘
              ▲                  ▲
              │ REST             │ WebSocket /v1/ws
              │ (commands +      │ (subscribed event stream)
              │  initial load)   │
              │                  │
        ┌─────┴──────────────────┴─────────────┐
        │              beeper-tui              │
        │                                      │
        │  ┌────────────┐    ┌──────────────┐  │
        │  │ apiClient  │    │  wsClient    │  │
        │  │ (REST)     │    │  (event sub) │  │
        │  └─────┬──────┘    └──────┬───────┘  │
        │        │                  │          │
        │        └────────┬─────────┘          │
        │                 ▼                    │
        │      ┌──────────────────────┐        │
        │      │  tea.Msg channel     │        │
        │      │  (REST results +     │        │
        │      │   WS events +        │        │
        │      │   keystrokes)        │        │
        │      └──────────┬───────────┘        │
        │                 ▼                    │
        │      ┌──────────────────────┐        │
        │      │  Update(model, msg)  │ ◀── pure reducer
        │      │  → new model + cmd   │
        │      └──────────┬───────────┘        │
        │                 ▼                    │
        │      ┌──────────────────────┐        │
        │      │  View(model) string  │ ◀── pure render
        │      └──────────────────────┘        │
        └──────────────────────────────────────┘
```

### Bubbletea's Elm pattern is the spine
All state mutations go through a pure `Update(model, msg) → (model, cmd)` reducer. Keystrokes, REST responses, and WS events all become `tea.Msg` values funneling into the same reducer. This eliminates the classic TUI bug of a WS event racing a keystroke — there's exactly one mutator.

### Subscription set is derived from the model
The WS subscription set is not managed imperatively. The reducer computes the desired subscription set from the model (which chats are visible, which is selected, what the user filtered to) and emits a `subscriptions.set` command whenever it changes. Declarative.

### Connection state is first-class
The model carries a `connection` field with three states: `Connecting`, `Connected`, `Disconnected`. The UI always reflects this honestly (status bar text or full-width banner). WS drops trigger a reconnect command from the reducer; nothing is hidden inside the client.

### What we deliberately do not do for v1
- **No persistent local database.** State lives in memory plus a thin JSON cache for warm start.
- **No backend abstraction layer.** `apiClient` and `wsClient` import the Beeper Go SDK directly. If a "standalone Matrix" variant ever happens, we extract an interface then. Not now.
- **No plugin system, no scripting hooks, no theming engine.** All future work.

## 4. UI and interaction model

### Layout
Two-pane, side by side:

- **Left pane:** chat list. Each row is `<name>  <account>  <unread-count>`. Sorted most-recent first. Selected row is highlighted.
- **Right pane:** message preview of the currently-selected chat. Day separators ("Yesterday", "Today"). Author label for group chats; omitted for 1:1s.
- **Bottom status bar:** filter state, key hints, connection status.

Account filtering is **not** a permanent column — it's an overlay invoked by pressing `a`. The status bar shows `Filter: All ▾ (a)` when unfiltered, or `Filter: iMessage ▾ (a)` when filtered.

### Preview behavior
The preview pane is **debounced live preview**: when selection lands on a chat, after ~150ms of dwell the preview is fetched (or read from cache) and rendered. Holding j to scroll fast does not trigger fetches. Previewing **does not** mark messages as read.

### Open / read mode
Pressing `enter` opens the selected chat in a full-screen reading view. This is when read state changes (we tell the Desktop API the user has read up to the latest visible message). `Esc` returns to the two-pane triage view.

### New-message behavior
When a WS event delivers a new message in any subscribed chat:
- The chat's unread counter increments
- The chat list re-sorts (most-recent first)
- If that chat happens to be the currently-previewed one, the preview pane updates live
- No bell, no flash, no OS notification — Beeper Desktop is already handling those

### Keybindings (v1)
Both vim-style and arrow keys work where applicable.

| Mode | Key | Action |
|---|---|---|
| List | `j` / `↓` | Move selection down |
| List | `k` / `↑` | Move selection up |
| List | `g g` | Jump to top |
| List | `G` | Jump to bottom |
| List | `enter` | Open selected chat (full-screen reading mode) |
| List | `a` | Open account filter overlay |
| List | `/` | Open search (placeholder in v1; full implementation in v1.1) |
| List | `q` | Show quit confirmation overlay (y to quit, n/Esc to dismiss) |
| List | `?` | Show help overlay |
| Reading | `j` / `↓` | Scroll messages down |
| Reading | `k` / `↑` | Scroll messages up |
| Reading | `g` / `G` | Jump to top / bottom |
| Reading | `esc` | Back to triage view |
| Reading | `q` | Show quit confirmation overlay |
| Overlay | `j/k` `↑/↓` | Navigate options |
| Overlay | `enter` | Select |
| Overlay | `esc` | Dismiss overlay |

### Modes are explicit and few
- **Triage** (the default two-pane view)
- **Reading** (full-screen chat)
- **Overlay** (account filter, help, quit confirm, future search) — overlays modal-dismiss to whatever mode launched them

Search is deferred to a later milestone within v1 (or v1.1). The `/` key reserves the binding and shows a "coming soon" placeholder so users don't form a different muscle memory and have to relearn.

## 5. Internal structure

### Go package layout

```
cmd/
  beeper-tui/
    main.go              # entry point, flag parsing, signal handling, bubbletea program init

internal/
  config/                # XDG config + cache paths, token discovery
    config.go
    config_test.go

  api/                   # REST client wrapping the Beeper Go SDK
    client.go            # apiClient struct, list/get methods
    client_test.go       # against fake HTTP server
    fixtures_test.go

  ws/                    # WebSocket client wrapping the Beeper Go SDK
    client.go            # connect, subscribe, event stream
    client_test.go
    events.go            # event type definitions / decoders

  ui/                    # bubbletea model + view (the UI layer)
    model.go             # Model struct, Init/Update/View
    update.go            # Update dispatch
    view.go              # rendering
    keys.go              # keybinding definitions
    panes/
      chatlist.go        # left pane rendering
      preview.go         # right pane rendering
      reading.go         # full-screen reading mode
      overlay.go         # account filter / help overlay
    update_test.go       # table-driven reducer tests

  state/                 # cache load/save, JSON schema
    cache.go
    cache_test.go

docs/
  superpowers/specs/
    2026-05-17-beeper-tui-design.md   # this file

go.mod
go.sum
README.md
LICENSE                  # MIT
```

### Key types (sketch)

```go
// internal/ui/model.go
type Model struct {
    Conn         ConnState
    Chats        []Chat
    Selected     int          // index into Chats (or a sortable key)
    Filter       AccountFilter
    Preview      PreviewState
    Mode         Mode         // Triage / Reading / Overlay
    Cache        *state.Cache
    ApiClient    api.Client
    WsClient     ws.Client
}

type Chat struct {
    ID          string
    Account     string       // "iMessage", "Signal", "Slack", etc.
    Name        string
    Unread      int
    LastMessage MessagePreview
    LastTs      time.Time
}

type PreviewState struct {
    ChatID   string
    Messages []Message       // most-recent-first or chronological — decide in implementation
    Loaded   bool
    Loading  bool
    Err      error
}
```

These will firm up during implementation; the spec doesn't pin every field.

### Data flow on launch (cold start)

1. `main` resolves config (XDG paths, token via SDK auto-discovery or env)
2. Bubbletea starts with an initial `Model{Conn: Connecting}`
3. `Init()` returns two commands: connect WS, REST-fetch chat list
4. REST response → `chatsLoaded` msg → reducer populates `Chats`, view renders the list
5. WS connect success → `wsConnected` msg → reducer sets `Conn: Connected`, emits `subscriptions.set` for the visible chats
6. WS events arrive → reducer updates `Chats` and `Preview` as appropriate
7. Cache snapshot is written on a debounced schedule (every ~5s of model dirty) and on graceful quit

### Data flow on warm start

1. `main` reads cache from disk → seeds `Model.Chats` from cached snapshot
2. View renders the cached list immediately (this is the <1s warm start)
3. In parallel, REST refetches and replaces the chat list when fresh data arrives; WS subscribes
4. Cache rewrite happens on quit and periodically

## 6. Error handling and resilience

### Failure modes and what the user sees

| Condition | What the TUI does |
|---|---|
| Beeper Desktop not running at launch | Show first-run onboarding screen (point to Settings → Developers); retry on `r` |
| WS connection drops | Status bar shows "Reconnecting…"; banner appears if still disconnected after 5s; exponential backoff (1, 2, 4, 8, 16, 30s max) |
| REST request fails (5xx / timeout) | Status bar shows transient error message; offer retry; cached data stays visible |
| Specific chat fetch fails | Preview pane shows "Couldn't load messages — press r to retry"; chat list unaffected |
| Cache file corrupt | Warn once in status bar; ignore cache and do a cold load |
| Token rejected (401) | Clear the cached token (env var); show onboarding screen |

### No silent retries
Every retry is either explicit (user presses `r`) or visible (status bar / banner shows we're reconnecting). The TUI never hides connection trouble from the user.

### Graceful shutdown
On `q`: show a confirm-on-quit overlay ("Quit? (y/n)"); `y` proceeds with shutdown, `n` or `Esc` dismisses. On confirmed quit, SIGINT, or SIGTERM: flush cache, close WS, exit 0. We never leave a half-written cache file.

## 7. Configuration and persistence

### Config file
Path: `$XDG_CONFIG_HOME/beeper-tui/config.toml` (macOS: `~/Library/Application Support/beeper-tui/config.toml` via `os.UserConfigDir`).

For v1, the config file is optional and minimal:
```toml
[api]
# Override the default 127.0.0.1:23373 if you need to.
# base_url = "http://localhost:23373"

[ui]
# Future home for theme, keybindings, etc. Empty for v1.
```

### Cache file
Path: `$XDG_STATE_HOME/beeper-tui/cache.json` (macOS: `~/Library/Caches/beeper-tui/cache.json` via `os.UserCacheDir`).

Contains:
- Chat list snapshot (id, name, account, last-message preview, unread count, last-ts)
- Last-selected chat ID (so warm start restores focus)
- Schema version (for future migration)

Cache is **purely an optimization.** Source of truth is always the API. If the cache is missing or unreadable, cold-load gracefully.

## 8. Testing strategy

### What we test, and how

| Layer | What | How |
|---|---|---|
| **Reducer** (`internal/ui`) | State transitions for every `tea.Msg` we handle | Pure unit tests, table-driven. No IO, no real bubbletea program needed. |
| **REST client** (`internal/api`) | Method correctness, error handling, retry behavior | Standard `httptest.Server` serving canned responses |
| **WS client** (`internal/ws`) | Connect, subscribe, decode events, reconnect | Test against a small `httptest` server hosting a real WS |
| **View** (`internal/ui`) | Render produces expected strings for known models | Snapshot tests on `View(model)` output |
| **Integration** | End-to-end against real Beeper Desktop | One test, gated behind `// +build integration`; not run in CI; you run it locally when validating a release |

### Why test the reducer in isolation
The reducer is the thing that has all the edge cases (WS event arriving for a chat we just filtered out, selection on a chat that got deleted server-side, etc.). Those are tedious to reproduce by hand against real Beeper. They're trivially easy to unit-test as `(state, event) → state` assertions.

We are not building a fake server to *replace* real Beeper. The integration test absolutely hits the real local API. The reducer tests just save us from manually clicking through twenty edge cases on every change.

## 9. Distribution

v1 ships as `go install github.com/<user>/beeper-tui@latest`. Anyone with Go installed can use it. Pre-built binaries via GitHub Releases are deferred until there's demand.

License: MIT.

## 10. Open questions to resolve during implementation

These don't need to block the design but should be made explicit during implementation:

- **Exact REST endpoints and field names** — confirm against the SDK during the first slice; this spec assumes the operations the Desktop API CLI exposes (`chats list`, `messages list`, etc.) map onto SDK methods cleanly.
- **WS event schema** — confirm the precise event types emitted (`new_message`, `message_edited`, `message_deleted`, `read_receipt`, `typing`, etc.) and which we actually need for v1.
- **Account identification** — the API likely uses account IDs, not human-readable names. We'll need a mapping from account ID → display label.
- **Group chat author rendering** — need a sender display name available per message; confirm SDK provides it without an extra round-trip.
- **Time handling** — local timezone for the day separators; confirm timestamp format from the SDK.

## 11. Roadmap beyond v1

| Milestone | Adds |
|---|---|
| **v1** | Read-only triage (this spec) |
| **v1.1** | Search across chats and messages |
| **v2** | Send text messages |
| **v3** | Attachments, reactions, replies, threads, edits, deletes |
| **v4+** | Pluggable themes, configurable keybindings, multi-account UX refinements |
