# M3: Live inbox via WebSocket — design

**Date:** 2026-05-23
**bd issues:** `beeper-tui-qbg.5` (WS client + reconnect), `beeper-tui-qbg.12` (apply events to UI), `beeper-tui-qbg.13` (connection status UX)
**Out of scope (separate spec):** `beeper-tui-qbg.15` warm-start from cache.

## Problem

The app is a **snapshot**. After the initial REST load, new messages never appear — you must restart to see them. For a triage tool you keep open, that's the core gap. M3 makes the inbox **live**: new/edited/deleted messages and chat changes flow in while the app runs.

## Why WebSocket (not polling)

Beeper Desktop exposes a documented live-events WebSocket (shipped v4.2.557, 2026-02-13 — after this issue was first deferred, which is why older notes claimed "no WS support"). It pushes events instantly, which is the experience we want. The Beeper Go SDK does **not** wrap it, so we hand-write a small client. Polling was considered and rejected: WS gives instant updates and the protocol is fully documented, removing the main risk (an unknown wire format).

## The documented protocol

- **Connect:** `ws://localhost:23373/v1/ws`, header `Authorization: Bearer <token>` (same token as REST).
- **Client → server** (only command we use):
  ```json
  {"type":"subscriptions.set","requestID":"r1","chatIDs":["*"]}
  ```
  `["*"]` = all chats; `[]` = pause; specific IDs = those only (`*` can't mix with IDs).
- **Server → control messages:**
  - `{"type":"ready","version":1,"chatIDs":[]}` — initial handshake.
  - `{"type":"subscriptions.updated","requestID":"r2","chatIDs":[...]}` — ack of a set.
  - `{"type":"error","requestID":"r2","code":"INVALID_PAYLOAD","message":"..."}`.
- **Server → domain events** (shared envelope):
  ```json
  {"type":"message.upserted","seq":42,"ts":1739320000000,"chatID":"chat_a","ids":["m1","m2"],"entries":[{"id":"m1", ...}]}
  ```
  Types: `message.upserted`, `message.deleted`, `chat.upserted`, `chat.deleted`. Upserts carry full `entries`; deletes are ID-only (`ids`). `seq` is a monotonic counter for gap detection.
- **Not documented:** ping/keepalive, reconnect guidance, and the exact fields inside `entries`. We design our own keepalive/reconnect (below); the `entries`/chat field shapes are confirmed during live testing and mapped to the existing `api.Message`/`api.Chat`.

## Decisions (settled during brainstorming)

| Decision | Choice | Rationale |
|---|---|---|
| Transport | WebSocket | Instant; protocol documented. |
| Library | `github.com/coder/websocket` | Modern, context-first, built-in `conn.Ping`. |
| Subscription scope | `["*"]` (all chats) | Triage wants every unread bump; localhost = cheap. |
| Event → reducer bridge | goroutine → channel → `waitForWSEvent` `tea.Cmd` | Idiomatic bubbletea; everything funnels through `Update`, so WS can't race keystrokes. |
| Auto-scroll on live msg in open chat | Only when already at bottom | Don't yank a user reading history. |
| Read-on-arrival | Mark read if chat open **and** at bottom | You're looking at it; otherwise bump unread. |
| Catch-up after reconnect | REST refetch + reconcile (snapshot resync); use `seq` to detect gaps | Standard "snapshot + stream"; always correct; replay-since-seq isn't documented. |
| Keepalive | Periodic WS ping/pong (`conn.Ping`) | RFC 6455 control frames — the proper mechanism; detects silently-dead sockets. |
| Reconnect backoff | `1, 2, 4, 8, 16, 30s` cap; `r` forces immediate retry (resets backoff) | Visible, never hidden. |

## Architecture

New package `internal/ws` (mirrors `internal/api`):
- `events.go` — typed envelope (`Event{Type, Seq, TS, ChatID, IDs, Entries}`) + control-message types + JSON decoder.
- `client.go` — connect, the read loop, ping loop, reconnect-with-backoff. Public surface:
  - constructor taking the bearer token + base URL,
  - `Events() <-chan Event` — a **typed** channel of ws events/state-changes. The `ws` package does **not** import bubbletea (same clean layering as `internal/api`); the UI's `waitForWSEvent` cmd reads `Event` values and wraps them into `tea.Msg`.
  - a `Close()` for graceful shutdown.

Data flow:

```
ws goroutine ──reads socket──> Go channel ──waitForWSEvent (tea.Cmd)──> tea.Msg ──> Update reducer ──> Model
```

- A goroutine runs the WS read loop so the UI never blocks on the network.
- It decodes each frame and pushes a typed `Event` onto the channel. Connection-state changes (connecting/connected/disconnected) are pushed as `Event`s too.
- `waitForWSEvent` (in the `ui` package) is a `tea.Cmd` that blocks on the channel, wraps one `Event` as a `tea.Msg`, and the reducer re-issues it — the documented bubbletea pattern for bridging the outside world into `Update`.

Model additions (`internal/ui/model.go`):
- `conn ConnState` — `Connecting | Connected | Disconnected`.
- `atBottom bool` (or derived from `msgOffset` + viewport height) to drive auto-scroll/read-on-arrival.

## Connection lifecycle & error handling (qbg.13)

- States surfaced honestly in the status bar: quiet when `Connected`, "Reconnecting…" while retrying, a clear message when down.
- On `ready` → set `Connected`, send `subscriptions.set ["*"]`.
- Ping every ~30s via `conn.Ping(ctx)`; a ping timeout or read error → `Disconnected` → reconnect with backoff.
- On reconnect: re-send `subscriptions.set`, then trigger a REST refetch of chats (and the open conversation) to reconcile missed events. A jump in `seq` confirms a gap occurred.
- Startup failure (Beeper Desktop not running / bad token): never trap the user — the REST cold-load list still shows, status bar shows disconnected, backoff keeps retrying, `r` forces a retry.

## Event → UI rules (qbg.12)

All rules run in the pure reducer as `(model, wsEventMsg) → model`.

- **`message.upserted`** — map `entries` → `api.Message`; update the chat's `Preview`/`LastActive`.
  - Unread: `IsFromMe` → no bump. `Muted`/`LowPriority` → update preview but no float-to-top (existing rule). Else → bump `Unread` + float-to-top via existing `sortChats`.
  - Open conversation: append the message; auto-scroll only if at bottom; if open **and** at bottom, treat as read (no bump).
  - **Dedup:** if the message `ID` already exists, reconcile in place — this confirms an M2 optimistic send (server echo) instead of double-rendering, and clears any `failedSends` entry.
- **`message.deleted`** — remove by `ID` from `messages` if the chat is open; best-effort preview update.
- **`chat.upserted`** — update `Title`/`Unread`/`Muted`/`LowPriority`/`LastActive`; re-sort. This is how reading on another device clears the unread badge here.
- **`chat.deleted`** — remove from `chats`; if it was open/selected, fall back gracefully (return to list / reselect neighbor via existing `reselectByID`).

## Testing

- **`internal/ws`:** an `httptest` server speaking WS (coder/websocket server side) sends `ready` then scripted events. Assert: client sends `subscriptions.set` after `ready`; decodes each envelope type onto the channel; reconnects with backoff when the server closes (inject a short backoff schedule in tests). No real Beeper.
- **Reducer:** table-driven `(model, wsEventMsg) → model` covering every event type and edge cases: event for unknown chat, message-from-me (no bump), muted/low-priority chat (no float), deleted open chat, optimistic-echo dedup, read-on-arrival when at bottom vs scrolled up.
- All automated tests use **synthetic data only** (e.g. "Alice", "hello"). The user runs live verification against real Beeper; any build-tagged integration test asserts counts/non-empty only, never printing content.

## Open items confirmed during live testing

- Exact JSON field names inside `message.upserted` `entries` → align `ws/events.go` decoding with `api.Message`/`api.Chat` mapping.
- Whether `chat.upserted` fires on read-elsewhere (expected) and carries the updated unread count.

## Wire-format findings (2026-06-10, live testing during qbg.5)

Confirmed against the real server; where these contradict the protocol sketch
above, the wire wins:

- **`ts` is a string, not an integer.** Strict integer decoding rejected every
  real event frame (the client silently dropped all events until fixed).
  `ws/events.go` now decodes `ts` leniently and never rejects an event over it.
- **`chat.upserted` carries no `entries`** (~155B, envelope + IDs only), so
  chat updates need a REST refetch or ID-only handling in qbg.12.
  `message.upserted` does carry full entries.
- The envelope also has an **`accountID`** field, and the server can emit
  **`reaction.added` / `reaction.removed`** (seen in Beeper's TS adapter
  types; we skip unknown types by design).
- **Mark-read and archive/unarchive fire `chat.upserted`**, which lets
  integration tests trigger events programmatically — see
  `TestIntegration_LiveEventRoundTrip` (no manual message sending needed).
- Beeper's own clients (CLI `watch`, chat adapter) send `subscriptions.set`
  on socket open without waiting for `ready` and omit `requestID`; our
  ready-then-set with `requestID` is also accepted and acked.
- **iMessage emits no events at all.** Per-bridge probe
  (`TestIntegration_BridgeEventMatrix`, mark-read trigger against one chat
  per network): Beeper (Matrix), Discord, Facebook/Messenger, Instagram,
  LinkedIn, Signal, Twitter/X and WhatsApp all emit; iMessage is silent —
  no `message.upserted`, no `chat.upserted`. Confirmed twice with real
  iMessage sends. Worth reporting upstream.

## Polling backstop (added during qbg.12)

Because of the iMessage gap, events alone cannot make the inbox dependable.
`internal/ui/poll.go` refetches the chat list and the open conversation every
30s, bounding staleness for silent bridges and any missed events. Background
refreshes are silent on failure, preserve the reader's scroll position, and
keep unconfirmed optimistic sends. Events still deliver instantly where
emission works.
