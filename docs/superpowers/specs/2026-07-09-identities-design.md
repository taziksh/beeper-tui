# Identities (person cards) — design

**bd epic:** `beeper-tui-dn0` (P2)  
**Date:** 2026-07-09  
**Status:** agreed direction; Phase A not yet implemented

## Problem

Beeper surfaces **network-scoped profiles** (contacts, chat participants,
message senders). The same human often appears as several profiles across
WhatsApp, Signal, iMessage, etc. The TUI has nowhere local to store **what you
know about a person** — preferred name, notes, context — independent of which
chat is open.

## Goals

- **Notes-first person cards:** remember things about people.
- **Manual links** from Beeper profiles/chats so a card can surface in context.
- **Local-only, private** storage. Sensitive by default.

## Non-goals (Phase A)

- Auto-linking / auto-merging people across networks.
- Inbox merge of multi-network DMs.
- Preferred-network routing or “message this person on X”.
- Network / relationship visualization (later, **check with user first**).
- Reading real contact/chat data into design docs, logs, tests, or exports.
- Cloud sync or sharing identities off-machine.

Later product ideas (viz, richer CRM fields) hang off the same durable model;
they are not required for Phase A.

## Concepts

| Term | Meaning |
|------|---------|
| **Profile** | Beeper `User` (or a single DM chat used as an anchor), scoped to one account/network. |
| **Identity** | Local person card: your name for them + notes (+ optional links). |
| **Link** | Manual association: one or more profiles/chats → one identity. |

Messaging stays chat-centric. Identities sit **beside** the inbox as memory,
not as a send path.

## Data model

```text
Identity
  id              uuid / stable local id
  display_name    your name for them
  notes           freeform text (markdown-ish later if useful)
  links[]         see ProfileLink
  created_at
  updated_at

ProfileLink
  kind            "user" | "chat"
  account_id      Beeper account id (required for kind=user)
  user_id         Beeper user id (kind=user)
  chat_id         Beeper chat id (kind=chat; typically type=single)
  linked_at
```

- One identity may have many links.
- One profile/chat should map to at most one identity (enforce on link).
- Free-floating identities (no links yet) are allowed.

### Storage

| What | Where | Lifetime |
|------|--------|----------|
| Identities + links + notes | Config dir: `identities.json` (mode `0600`) | Durable, user-authored |
| Optional profile display cache | Cache dir only if needed later | Rebuildable; not source of truth |

Path helpers already exist via `config.XDGConfigDir()`  
(e.g. macOS: `~/Library/Application Support/beeper-tui/`).

Do **not** put identities in the warm-start chat cache (`state.Cache`); cache
schema bumps must not wipe notes.

### Privacy

- Treat notes and link keys as sensitive local data.
- File mode `0600`; never log contents or dump real identities in tests/debug.
- Automated tests use synthetic people only (“Alice”, “Bob”).
- Any feature that scrapes real contacts, builds graphs from real social
  structure, or exports identity data: **ask the user first**.

## Phase A — product surface

1. **Package `internal/identity`**
   - Load / save `identities.json`.
   - CRUD: create, update display name / notes, delete.
   - Link / unlink profile or chat.
   - Resolve: `(accountID, userID)` → identity; `chatID` → identity.

2. **Stable anchors from the API layer**
   - For `type == single` chats, keep enough peer identity to link (at least
     peer `userID` from participants when available; otherwise `chatID` alone).
   - Optional: plumb `SenderID` on messages later; not required for DM cards.

3. **TUI**
   - From a conversation (or selected chat): open identity card or create one.
   - Edit display name and notes; save on confirm.
   - Link current chat / peer to this identity (manual).
   - If already linked, open the existing card.
   - Keybinding: TBD at implement time (reserve something like `I` or `n`;
     avoid colliding with insert/search).

4. **No auto-suggest, no inbox changes** in Phase A.

## Later (not scheduled)

- Soft match suggestions (phone/email) — still confirm before link.
- Structured fields (birthday, tags, how you met).
- Full-text search over notes / identity names.
- Network visualization over the identity graph — **requires explicit user OK**
  before using real data or shipping a viz that exposes relationship structure.

## Acceptance (Phase A)

- [ ] Create an identity with a display name and notes; survives restart.
- [ ] From a single chat, link to that identity (or create+link).
- [ ] Re-open the same chat → same identity card.
- [ ] Unlink and delete work without orphaning the file format.
- [ ] Unit tests on load/save/resolve with synthetic data only.
- [ ] No real user content in logs, fixtures, or docs.

## Open decisions (resolve at implement)

- Exact keybinding and whether notes edit is inline vs overlay editor.
- Whether `chat` links alone are enough for v1 vs requiring peer `userID`.
- JSON vs SQLite if the file grows large (start with JSON).
