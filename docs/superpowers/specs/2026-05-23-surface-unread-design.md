# Surface unread messages — design

**bd issue:** `beeper-tui-jfs` (P1)
**Date:** 2026-05-23

## Problem

The chat list shows a `*` mark and a right-aligned unread count, but unread
chats are hard to triage at a glance. Three concrete frictions, in the user's
words:

1. **Can't spot them** — the `*` and count blend into every other row.
2. **They're scattered** — unread chats sit anywhere in the list, mixed among
   read ones.
3. **Lost inside a conversation** — on opening a chat there's no way to tell
   which messages are actually new versus already seen.

## Scope

This pass addresses all three frictions. Explicitly **out of scope** (deferred):

- Filter/toggle to show only unread chats (`u` key) — fast-follow after this.
- Jump-to-next-unread key — user did not want it.
- Per-message read tracking beyond the SDK's `IsUnread` flag (no last-read
  timestamp plumbing).

## Design

### 1. Visual emphasis (list rows + message rows)

A single themeable **ANSI accent color** (yellow) signals "unread,"
consistently across both views. Bold stays reserved for the selected row, so the
two signals are orthogonal and compose:

| State                   | Rendering              |
|-------------------------|------------------------|
| read, not selected      | plain                  |
| read, selected          | bold                   |
| unread, not selected    | accent-colored `●` glyph + accent-colored count |
| unread, selected        | bold **and** accent-colored |

Implementation: define the accent as a `lipgloss` style using an ANSI color
(not truecolor) so it respects the user's terminal theme. The unread glyph
changes from `*` to a filled `●`. Apply the accent style to the glyph + count
columns when `c.Unread > 0`. The existing bold-selected style is applied
independently on top.

### 2. Float unread to top

Chats are currently rendered in API order (no UI sort). Sort the chat slice so
unread chats cluster at the top:

- **Primary key:** `Unread > 0` (unread before read).
- **Secondary key:** `LastActive` descending (most recent first), within each
  group.

Sorting happens wherever `m.chats` is assigned — currently the `chatsLoadedMsg`
case in `update.go`. A small `sortChats([]api.Chat)` helper keeps it testable
and reusable for the future live-update path.

**Selection stability:** because rows can reorder, the cursor must track the
selected chat by **ID, not row index**. When re-sorting an existing list,
capture the selected chat's ID before the sort and restore `m.selected` to that
chat's new index afterward (fall back to clamping if it's gone). On the initial
load there's no prior selection, so `selected` stays 0.

### 3. Per-message new marker + auto-scroll (conversation view)

The SDK exposes `shared.Message.IsUnread` ("True if the message is unread for
the authenticated user. May be omitted."). We plumb it through:

- Add `IsUnread bool` to `api.Message`.
- Set it in `mapMessage` from `m.IsUnread`.

In the conversation view, each row with `IsUnread == true` gets an
accent-colored left marker (a `▎` bar) — the same accent color as the list, so
"new" reads identically in both places.

**Auto-scroll:** on `messagesLoadedMsg`, if any loaded message is unread, set
`msgOffset` so the **first** unread message sits at (or near) the top of the
viewport, instead of the current scroll-to-bottom behavior. If nothing is
unread, keep scroll-to-bottom.

**Sequencing / read-state timing:** `ListMessages` fetches `IsUnread` as it was
before the chat is marked read, and our local `m.messages` slice holds those
flags for the lifetime of the open conversation. So the markers persist while
you read, even though `markReadCmd` runs on open. This is intended — you keep
seeing what was new until you leave and re-open.

**`IsUnread` omitted:** some networks may not set the flag. We trust it as-is;
if it's blank everywhere we simply show no markers (no "last N" heuristic
fallback unless a real network proves it necessary).

## Components touched

- `internal/api/types.go` — add `Message.IsUnread`.
- `internal/api/messages.go` — map `IsUnread` in `mapMessage`.
- `internal/ui/view.go` — accent style + `●` glyph + colored count in
  `renderList`; `▎` marker in `renderConversation`.
- `internal/ui/update.go` — sort on `chatsLoadedMsg`; first-unread `msgOffset`
  on `messagesLoadedMsg`.
- New `sortChats` helper (likely `internal/ui/nav.go` or a small new file) with
  selection-by-ID stability.

## Testing

- `sortChats`: unread-before-read, recency within group, stable on empty/single.
- Selection stability: given a selected chat ID, re-sort puts the cursor on the
  same chat; gone-chat falls back to a clamped index.
- Auto-scroll: `msgOffset` lands on the first unread; scroll-to-bottom when none
  unread.
- `mapMessage`: `IsUnread` carried through.
- View rendering (golden/string assertions consistent with existing
  `view_test.go`): unread row carries glyph + accent; selected-unread carries
  both bold and accent; convo unread row carries the marker.
