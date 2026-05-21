# beeper-tui — read→reply redesign (supersedes the v1 UI design)

This supersedes the UI/layout sections of [2026-05-17-beeper-tui-design.md](2026-05-17-beeper-tui-design.md). The backend choice (local Beeper Desktop API), config, and cache layers from that spec still stand. What changed: the **goal**, the **layout**, and the **keymap**.

## Why this redesign

The original spec optimized for *read-only triage* (a two-pane chat list + message preview). Through use and discussion, the real goal clarified: **read and reply to texts from the terminal.** The chat list is just the picker; reading and replying is the point. We also confirmed (2026-05-21) that **no existing terminal client works turn-key for Beeper** — iamb (no token login), gomuks (E2EE/recovery-key wall), matrix-commander (CLI not TUI), new-gomuks (web not terminal). See [[existing-tools-ruled-out]]. Building on the **Beeper Desktop API** is the only path that's terminal + private + reliable, because the Desktop app hands us **already-decrypted** messages — no E2EE wall.

## Goal

Open `beeper-tui` → pick a conversation → **read its real messages** → **type and send a reply** — all in the terminal, keyboard-only, vim-native.

## Milestones (value-based, not technical layers)

- **M1 — Read:** launch → reach a conversation → see its real messages on screen. (Uses `ListChats` ✅ + `ListMessages` ✅ from Phase 2; needs the UI.)
- **M2 — Reply:** in a conversation, compose and send a plain-text message. (Needs a `Send` API wrapper + the compose flow.)

Each milestone must produce something the user can actually *do*, not just an internal layer.

## Backend

Local Beeper Desktop API at `http://127.0.0.1:23373` (REST), token via `BEEPER_ACCESS_TOKEN`. Messages arrive **decrypted** (Desktop app handles E2EE). No WebSocket in this scope — live updates are deferred; the user refreshes/re-opens to pull new messages. (WS remains deferred per the original spec.)

## Layout — full-width conversation + toggleable list

One coherent view, not a permanent split:

- **Default:** the **conversation fills the screen** — message history + a compose line at the bottom + a one-line status bar.
- **Hotlist bar:** the status bar shows chats with unread, compactly (e.g. unread chat names/counts). Ambient "what to read next" awareness without a permanent list.
- **Toggleable list:** `<leader>l` slides a narrow chat list in on the left; press again to reclaim full width. (Proven pattern — nchat's `Ctrl-l`.) The conversation is **never forced narrow** — the list is opt-in.
- **Fuzzy switch:** `<leader>ff` opens a fuzzy chat picker (type a few letters → Enter → jump). This is the primary navigation and the only thing that scales to ~900+ chats — you never scroll a giant list.

### Modal (vim)

- **NORMAL** mode (default): navigate/read/run commands. Mode shown bottom-left.
- **INSERT** mode (`i`): type a reply. `Enter` sends; `Esc` returns to NORMAL.
- This is the proven vim-chat model (weechat-vimode, iamb): in NORMAL mode every key is free for commands because typing only happens in INSERT.

## Keymap (v1)

Designed around the user's actual Neovim/Telescope muscle memory (`<space>` leader; `ff`/`fg` find/grep) — see [[user-vim-telescope-setup]]. Nothing is a disguised chat-client convention.

| Action | Key | Origin |
|---|---|---|
| Scroll messages | `j` / `k`, `gg` / `G` | core vim motion |
| Compose reply | `i` | vim insert |
| Send | `Enter` (in INSERT) | — |
| Back to NORMAL | `Esc` | vim |
| **Find / switch chat (fuzzy)** | **`<leader>ff`** | mirrors user's Telescope find_files |
| **Grep messages** (deferred to later) | **`<leader>fg`** | mirrors user's Telescope live_grep — same keys |
| Toggle chat list | `<leader>l` | leader-prefixed toggle (modern vim) |
| Next / prev unread | `]u` / `[u` | vim bracket "next/prev" idiom |
| Quit | `:q` | vim ex-command |
| Help | `?` | vim |

Leader = `<space>`.

## What's deferred (explicitly NOT in M1/M2)

- Live updates / WebSocket (refresh-to-pull for now)
- Message grep/search (`<leader>fg` reserved, built later)
- Reactions, attachments, threads, edits, replying-to-a-specific-message
- Account filtering as a separate feature (fuzzy-find covers navigation)
- Read receipts nuance beyond mark-as-read on open

## Read-state behavior

Opening a conversation marks it read (via `MarkRead` ✅). Fuzzy-previewing in the picker does not.

## Testing

Pure-function reducer (bubbletea `Update`) unit-tested with table-driven tests; render helpers tested by substring assertions; one build-tagged integration test against the real Desktop API. **Per the user's hard data rule, all automated tests use synthetic/fake data only — never the user's real chats. Live verification is run by the user, not surfaced into any AI conversation.** See [[never-surface-real-user-data]].

## Tech

Go 1.26.3, bubbletea v2 (`charm.land/bubbletea/v2`) + lipgloss v2, on top of Phase 2's `internal/api` (`ListChats`, `ListMessages`, `MarkRead`) and Phase 1's `internal/config` + `internal/state`.
