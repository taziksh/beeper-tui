# beeper-tui — M2 (Reply) design

Implements **M2 — Reply** from [the read→reply redesign](2026-05-21-beeper-tui-read-reply-design.md). M1 (Read) is complete and tagged `milestone-1-read`. This spec covers only the reply half: composing and sending a plain-text message from inside a conversation.

## Goal

Inside an open conversation, press `i` to enter INSERT, type a plain-text reply, press `Enter` to send it, `Esc` to return to NORMAL. The sent message appears immediately (optimistically); if the send fails it is marked so you can see it didn't go.

## Scope

In:
- INSERT mode within a conversation (a third `Mode`).
- A `SendMessage` API wrapper over the Beeper Desktop SDK's `Messages.Send`.
- Optimistic display of the sent message with a failure marker on error.
- Open-at-bottom (newest) when entering a conversation.

Explicitly out (deferred, unchanged from the redesign spec):
- Attachments, reactions, threads, edits.
- Reply-to-a-specific-message (the SDK supports `ReplyToMessageID`, but not in M2).
- Live updates / WebSocket — no incoming messages stream in; the user re-opens to pull new ones.
- The grand layout (hotlist bar, `<leader>l` list toggle, `<leader>ff` fuzzy switch) — that's M1.5.

## Decisions (confirmed with user, 2026-05-22)

| Decision | Choice |
|---|---|
| Conversation opens at | **Bottom** (newest message + the reply affordance in view) |
| Compose line visibility | **Only in INSERT** — no compose line in NORMAL; it appears when you press `i` |
| Mode after `Enter` sends | **Back to NORMAL** — one deliberate message at a time; press `i` again for the next |
| Send display | **Optimistic + fail mark** — message shows instantly; `! send failed` suffix on error |

## API layer

### `internal/api/send.go`

```go
// SendMessage sends a plain-text message to a chat.
func (c *Client) SendMessage(ctx context.Context, chatID, text string) error
```

- Wraps `c.sdk.Messages.Send(ctx, chatID, beeperdesktopapi.MessageSendParams{Text: <opt>(text)})`.
- The SDK returns a `MessageSendResponse{PendingMessageID, ...}` — the send is confirmed asynchronously, not echoed back as a finished message. M2 **ignores** the pending id; the optimistic UI is what the user sees. Only success/error matters here.
- Wraps errors as `fmt.Errorf("api: send message to %s: %w", chatID, err)`, matching the other api wrappers.

### Message ordering — sort oldest-first in `ListMessages`

The SDK documents `Messages.List` only as *"Sorted by timestamp"* — it does not state the **direction**, and we can't probe the live API (hard data rule). So rather than depend on the API's order, `ListMessages` **explicitly sorts the mapped slice by `Timestamp` ascending (oldest-first)** before returning. This makes newest-at-bottom deterministic, so open-at-bottom and optimistic-append (which appends to the end) are both correct regardless of what the API returns. This also pins down M1's ordering, which currently trusts the API's unspecified order.

## Model — `internal/ui/model.go`

- Add `ModeInsert` to the `Mode` enum (the existing comment already reserves its slot).
- Add `input string` — the in-progress draft.
- Add `failedSends map[string]bool` — keyed by a local id; marks optimistic messages whose send errored.
- Add a small counter (`localSeq int`) to mint synthetic ids `local:1`, `local:2`, … for optimistic messages before the server assigns real ones.

## Key handling — `internal/ui/nav.go`

NORMAL + conversation:
- `i` → `ModeInsert`. `input` starts empty.

INSERT:
- Printable runes append to `input`; Backspace removes the last rune.
- `Enter` with **empty** `input` → no-op.
- `Enter` with text → optimistic send (below), then return to `ModeConversation` NORMAL.
- `Esc` → return to NORMAL, discarding the in-progress draft.

Optimistic send on `Enter`:
1. Mint `id := "local:N"`.
2. Append `Message{ID:id, ChatID:current, SenderName:"You", Text:input, Timestamp:time.Now(), IsFromMe:true}` to `m.messages`.
3. Clear `input`, jump scroll to bottom so the new line is visible.
4. Set mode back to NORMAL.
5. Return a command (`sendMessageCmd`) tagged with `id`.

The existing `handleKey` switches on key then mode; INSERT routing fits the same shape. Printable-rune handling reads the key string from `tea.KeyPressMsg` (single-rune keys append; named keys like `enter`/`esc`/`backspace` are handled explicitly).

## Commands & post-send — `internal/ui/messages.go`

- New `sendResultMsg struct { localID string; err error }`.
- New `sendMessageCmd(chatID, localID, text string) tea.Cmd` — calls `client.SendMessage`, returns `sendResultMsg{localID, err}`.
- `Update` handles `sendResultMsg`: on `err != nil`, set `failedSends[localID] = true`. On success, no visible change (the optimistic line stays).

## View — `internal/ui/view.go`

- `renderConversation`:
  - In INSERT, render a compose line above the status bar: `> ` + `input` + a cursor block. In NORMAL, no compose line (more room for messages).
  - A message whose `ID` is in `failedSends` renders a `! send failed` suffix.
  - On open, scroll position starts at bottom (newest), so `msgOffset` is set to `maxMsgOffset()` when messages load / conversation opens.
- `convStatusBar` reflects mode: `NORMAL  j/k scroll · i reply · esc back · q quit` vs `INSERT  enter send · esc cancel`.

### Open-at-bottom detail

`openSelected` sets `loadingMsgs = true`; the actual messages arrive via `messagesLoadedMsg`. Set `msgOffset = maxMsgOffset()` (clamped) when messages load for the current chat, so the conversation lands on the newest message.

## Testing (synthetic data only — per the hard data rule)

All automated tests use fake/synthetic data; live verification is the user's, never surfaced into an AI conversation. See the read→reply spec's testing section.

- **api** (`send_test.go`): `SendMessage` success and error paths against a stub SDK/HTTP, mirroring the existing `markread_test.go` pattern.
- **reducer** (`update_test.go` / `nav_test.go`): `i` → INSERT; typing builds `input`; Backspace; `Enter` on empty = no-op; `Enter` on text = optimistic append + `input` cleared + NORMAL + a command issued; `Esc` discards draft → NORMAL; `sendResultMsg{err}` marks `failedSends`.
- **view** (`view_test.go`): INSERT shows `> draft` + cursor; NORMAL shows no compose line; a failed id renders `! send failed`; messages-loaded lands at bottom.

## Tech

Go (unchanged), bubbletea v2 (`charm.land/bubbletea/v2`) + lipgloss v2, on `internal/api` (now gaining `SendMessage`) and the existing M1 UI. `bd` (beads) tracks the work, not TaskCreate, per AGENTS.md.

## Out-of-band cleanup

The current `bd ready` list is stale v1 "read-only triage" issues (warm-start cache, account filter overlay, websocket) superseded by the read→reply redesign. Before/while planning M2, defer or close those and create an M2 epic with child issues (api wrapper, INSERT reducer, compose-line view, optimistic+fail, tests).
