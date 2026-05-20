# Phase 2 — REST Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `internal/api`, a tested Go package that talks to the local Beeper Desktop API over REST — listing chats, listing a chat's messages, and marking a chat read — and wire it into `main.go` so the binary prints the user's real chat list instead of placeholder status.

**Architecture:** A thin wrapper around the official Beeper Go SDK (`github.com/beeper/desktop-api-go/v5`). The wrapper exposes small, intention-revealing methods (`ListChats`, `ListMessages`, `MarkRead`) that return our own domain types (`api.Chat`, `api.Message`) — the verbose SDK-generated types never leak past this package boundary. Unit tests point the SDK at an `httptest.Server` serving canned Beeper-shaped JSON (captured from the real API), so the full path HTTP→SDK-parse→our-mapping is exercised without needing Beeper running. One build-tagged integration test hits the real local API.

**Tech Stack:**
- Go 1.26.3
- `github.com/beeper/desktop-api-go/v5` (official SDK; first third-party dependency)
- stdlib `net/http/httptest` for unit tests
- bd CLI for issue tracking

**bd issue covered:** `beeper-tui-qbg.4` (T1: REST client). NOTE: T2 (WebSocket) was deferred to v1.x — see `beeper-tui-qbg.5`. This plan is REST only.

**Module path:** `github.com/taziksh/beeper-tui`

**Prerequisite:** A valid `BEEPER_ACCESS_TOKEN` must be set in the environment for the integration test and the `main.go` smoke run. Beeper Desktop must be running with the Desktop API enabled. (The user has confirmed both.)

---

## Captured API facts (from live API, 2026-05-20)

`GET /v1/chats?limit=N` returns:
```json
{
  "items": [
    {
      "id": "...", "accountID": "...", "network": "WhatsApp",
      "title": "Hiking Group", "type": "group",
      "unreadCount": 82, "unreadMentionsCount": 0,
      "lastActivity": "2026-05-19T...Z",
      "preview": { /* last-message preview object */ },
      "participants": { /* ... */ }
    }
  ],
  "hasMore": true,
  "oldestCursor": "...", "newestCursor": "..."
}
```
Auth: `Authorization: Bearer <token>` required (401 without). `network` is the human label ("WhatsApp", "Signal", "iMessage"); `accountID` is the internal id.

---

## File Structure

| File | Created in task | Responsibility |
|---|---|---|
| `go.mod` / `go.sum` | 1 | Gains the SDK dependency |
| `internal/api/types.go` | 2 | Domain types `Chat`, `Message` (decoupled from SDK) |
| `internal/api/client.go` | 1, 3 | `Client` struct, `New(config.Config)` constructor |
| `internal/api/chats.go` | 4, 5, 6 | `ListChats`, pagination, SDK→domain mapping |
| `internal/api/messages.go` | 7 | `ListMessages` |
| `internal/api/markread.go` | 8 | `MarkRead` |
| `internal/api/client_test.go` | 3 | Constructor tests |
| `internal/api/chats_test.go` | 4,5,6 | httptest-based ListChats tests |
| `internal/api/messages_test.go` | 7 | httptest-based ListMessages tests |
| `internal/api/markread_test.go` | 8 | httptest-based MarkRead test |
| `internal/api/integration_test.go` | 9 | Build-tagged real-API test |
| `cmd/beeper-tui/main.go` | 10 | Calls ListChats, prints the real chat list |

---

## Conventions

- Conventional Commits (`feat:`, `test:`, `chore:`), one commit per task.
- Test names `Test<Method>_<Scenario>`.
- Domain types use plain Go types (`string`, `int`, `time.Time`) — no SDK or `param.Opt` types in the public surface of `internal/api`.
- Errors from the SDK are wrapped with context using `fmt.Errorf("api: <what>: %w", err)`.

---

## Task 1 — Add the SDK dependency and a Client skeleton

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/api/client.go`

- [ ] **Step 1: Add the dependency**

Run:
```bash
cd /Users/tazik/Projects/beeper-tui
go get github.com/beeper/desktop-api-go/v5@latest
```
Expected: `go.mod` gains a `require github.com/beeper/desktop-api-go/v5 vX.Y.Z` line and `go.sum` is populated.

- [ ] **Step 2: Create the Client skeleton**

Create `internal/api/client.go`:

```go
package api

import (
	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
	"github.com/beeper/desktop-api-go/v5/option"

	"github.com/taziksh/beeper-tui/internal/config"
)

// Client wraps the Beeper Desktop SDK with intention-revealing methods that
// return our own domain types.
type Client struct {
	sdk beeperdesktopapi.Client
}

// New constructs a Client from resolved config. The SDK reads the bearer token
// and base URL we pass; nothing else in the app touches the SDK directly.
func New(cfg config.Config) *Client {
	sdk := beeperdesktopapi.NewClient(
		option.WithAccessToken(cfg.Token),
		option.WithBaseURL(cfg.BaseURL),
	)
	return &Client{sdk: sdk}
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: clean build. If the SDK's `NewClient` returns a value vs pointer differently than above, or `option` import path differs, run `go doc github.com/beeper/desktop-api-go/v5.NewClient` and `go doc github.com/beeper/desktop-api-go/v5/option` to confirm exact signatures and adjust the struct field type (`sdk beeperdesktopapi.Client` vs `*beeperdesktopapi.Client`) accordingly. Report any deviation from the code above in your report.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/api/client.go
git commit -m "feat(api): add Beeper SDK dependency and Client skeleton"
```

---

## Task 2 — Domain types

**Files:**
- Create: `internal/api/types.go`

- [ ] **Step 1: Define the domain types**

Create `internal/api/types.go`:

```go
package api

import "time"

// Chat is our decoupled view of a Beeper chat — only the fields the TUI needs.
type Chat struct {
	ID         string
	AccountID  string
	Network    string // human label: "WhatsApp", "Signal", "iMessage"
	Title      string
	Type       string // "single" | "group" | etc.
	Unread     int
	LastActive time.Time
	Preview    string // plain-text last-message preview, may be empty
}

// Message is our decoupled view of a single message.
type Message struct {
	ID         string
	ChatID     string
	SenderName string
	Text       string
	Timestamp  time.Time
	IsFromMe   bool
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add internal/api/types.go
git commit -m "feat(api): add Chat and Message domain types"
```

---

## Task 3 — Constructor test

**Files:**
- Create: `internal/api/client_test.go`

- [ ] **Step 1: Write the test**

Create `internal/api/client_test.go`:

```go
package api_test

import (
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

func TestNew_ReturnsNonNilClient(t *testing.T) {
	c := api.New(config.Config{
		Token:   "test-token",
		BaseURL: "http://127.0.0.1:23373",
	})
	if c == nil {
		t.Fatal("New() returned nil")
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/api/... -run TestNew -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/client_test.go
git commit -m "test(api): cover Client constructor"
```

---

## Task 4 — ListChats happy path (single page)

This is the core of the phase. We point the SDK at an httptest server and verify our mapping.

**Files:**
- Create: `internal/api/chats.go`
- Create: `internal/api/chats_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/api/chats_test.go`:

```go
package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

// singlePageChatsJSON mirrors the real /v1/chats response shape captured from
// the live API on 2026-05-20.
const singlePageChatsJSON = `{
  "items": [
    {
      "id": "chat-1", "accountID": "acc-wa", "network": "WhatsApp",
      "title": "Hiking Group", "type": "group",
      "unreadCount": 82, "unreadMentionsCount": 0,
      "lastActivity": "2026-05-19T12:00:00Z",
      "preview": {"text": "see you there"}
    },
    {
      "id": "chat-2", "accountID": "acc-sig", "network": "Signal",
      "title": "Bob", "type": "single",
      "unreadCount": 0, "unreadMentionsCount": 0,
      "lastActivity": "2026-05-18T09:30:00Z",
      "preview": {"text": "ok!"}
    }
  ],
  "hasMore": false,
  "oldestCursor": "c-old", "newestCursor": "c-new"
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return api.New(config.Config{Token: "test", BaseURL: srv.URL})
}

func TestListChats_MapsSinglePage(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(singlePageChatsJSON))
	})

	chats, err := client.ListChats(context.Background())
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	if len(chats) != 2 {
		t.Fatalf("got %d chats, want 2", len(chats))
	}
	if chats[0].Title != "Hiking Group" {
		t.Errorf("chats[0].Title = %q, want %q", chats[0].Title, "Hiking Group")
	}
	if chats[0].Network != "WhatsApp" {
		t.Errorf("chats[0].Network = %q, want WhatsApp", chats[0].Network)
	}
	if chats[0].Unread != 82 {
		t.Errorf("chats[0].Unread = %d, want 82", chats[0].Unread)
	}
	if chats[0].Preview != "see you there" {
		t.Errorf("chats[0].Preview = %q, want 'see you there'", chats[0].Preview)
	}
	if chats[0].LastActive.IsZero() {
		t.Error("chats[0].LastActive is zero, want parsed timestamp")
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/api/... -run TestListChats_MapsSinglePage`
Expected: build failure — `client.ListChats undefined`.

- [ ] **Step 3: Implement ListChats + mapping**

Create `internal/api/chats.go`:

```go
package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// ListChats fetches all chats, following cursor pagination to completion,
// and returns them as domain Chats sorted by the API's default order.
func (c *Client) ListChats(ctx context.Context) ([]Chat, error) {
	pager, err := c.sdk.Chats.List(ctx, beeperdesktopapi.ChatListParams{})
	if err != nil {
		return nil, fmt.Errorf("api: list chats: %w", err)
	}

	var out []Chat
	for _, item := range pager.Items {
		out = append(out, mapChat(item))
	}
	return out, nil
}

func mapChat(c beeperdesktopapi.ChatListResponse) Chat {
	return Chat{
		ID:         c.ID,
		AccountID:  c.AccountID,
		Network:    c.Network,
		Title:      c.Title,
		Type:       string(c.Type),
		Unread:     int(c.UnreadCount),
		LastActive: c.LastActivity,
		Preview:    c.Preview.Text,
	}
}
```

**IMPORTANT for the implementer:** The exact field names and types on `beeperdesktopapi.ChatListResponse` (e.g., `UnreadCount` may be `int64`, `LastActivity` may be a `time.Time` or a string needing parse, `Preview` may be a nested struct with a different field than `.Text`, `Type` may be a typed string) must be verified with `go doc github.com/beeper/desktop-api-go/v5.ChatListResponse`. Adjust `mapChat` to match the real struct. If `Preview` is a complex type, extract the plain-text body; if no text is available, leave `Preview` empty. Report the actual struct shape in your report so later tasks stay consistent.

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/api/... -run TestListChats_MapsSinglePage -v`
Expected: PASS. If the SDK rejects the httptest response shape, inspect the SDK's expected envelope and adjust the test JSON to match the real shape (it should match what the live API returns).

- [ ] **Step 5: Commit**

```bash
git add internal/api/chats.go internal/api/chats_test.go
git commit -m "feat(api): add ListChats with SDK-to-domain mapping"
```

---

## Task 5 — ListChats pagination (multi-page)

The SDK's `Chats.List` returns a cursor pager. Task 4 only read the first page's `.Items`. This task makes ListChats follow the cursor to completion.

**Files:**
- Modify: `internal/api/chats.go`
- Modify: `internal/api/chats_test.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/api/chats_test.go`:

```go
func TestListChats_FollowsPagination(t *testing.T) {
	page1 := `{"items":[{"id":"a","network":"Signal","title":"A","type":"single","unreadCount":0,"lastActivity":"2026-05-19T12:00:00Z","preview":{"text":""}}],"hasMore":true,"newestCursor":"cur1","oldestCursor":"old1"}`
	page2 := `{"items":[{"id":"b","network":"Signal","title":"B","type":"single","unreadCount":0,"lastActivity":"2026-05-18T12:00:00Z","preview":{"text":""}}],"hasMore":false,"newestCursor":"cur2","oldestCursor":"old2"}`

	var calls int
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls == 0 {
			_, _ = w.Write([]byte(page1))
		} else {
			_, _ = w.Write([]byte(page2))
		}
		calls++
	})

	chats, err := client.ListChats(context.Background())
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	if len(chats) != 2 {
		t.Fatalf("got %d chats across pages, want 2 (calls=%d)", len(chats), calls)
	}
	if chats[0].ID != "a" || chats[1].ID != "b" {
		t.Errorf("got IDs %q,%q want a,b", chats[0].ID, chats[1].ID)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/api/... -run TestListChats_FollowsPagination`
Expected: FAIL — only 1 chat returned (pagination not followed).

- [ ] **Step 3: Switch to auto-paging**

Update `ListChats` in `internal/api/chats.go` to use the SDK's auto-paging iterator instead of reading a single page:

```go
func (c *Client) ListChats(ctx context.Context) ([]Chat, error) {
	iter := c.sdk.Chats.ListAutoPaging(ctx, beeperdesktopapi.ChatListParams{})
	var out []Chat
	for iter.Next() {
		out = append(out, mapChat(iter.Current()))
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("api: list chats: %w", err)
	}
	return out, nil
}
```

**IMPORTANT for the implementer:** Confirm the auto-paging method is named `ListAutoPaging` and that the iterator exposes `Next() bool`, `Current() T`, `Err() error` via `go doc github.com/beeper/desktop-api-go/v5.ChatService.ListAutoPaging` and the pagination package. Stainless SDKs follow this pattern but verify. Adjust if the names differ.

- [ ] **Step 4: Run — confirm both pagination tests pass**

Run: `go test ./internal/api/... -run TestListChats -v`
Expected: both `TestListChats_MapsSinglePage` and `TestListChats_FollowsPagination` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/chats.go internal/api/chats_test.go
git commit -m "feat(api): ListChats follows cursor pagination to completion"
```

---

## Task 6 — ListChats error handling

**Files:**
- Modify: `internal/api/chats_test.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/api/chats_test.go`:

```go
func TestListChats_PropagatesAuthError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})

	_, err := client.ListChats(context.Background())
	if err == nil {
		t.Fatal("ListChats() error = nil, want non-nil on 401")
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/api/... -run TestListChats_PropagatesAuthError -v`
Expected: PASS already (the auto-paging iterator surfaces the 401 through `iter.Err()`, which `ListChats` wraps and returns). If it does NOT pass — e.g., the iterator swallows the error — adjust `ListChats` so a non-2xx response becomes a returned error, then re-run.

- [ ] **Step 3: Commit**

```bash
git add internal/api/chats_test.go
git commit -m "test(api): ListChats surfaces auth errors"
```

---

## Task 7 — ListMessages

**Files:**
- Create: `internal/api/messages.go`
- Create: `internal/api/messages_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/api/messages_test.go`:

```go
package api_test

import (
	"context"
	"net/http"
	"testing"
)

const messagesJSON = `{
  "items": [
    {"id":"m1","chatID":"chat-1","text":"hey","timestamp":"2026-05-19T10:00:00Z","isSender":false,"senderName":"Bob"},
    {"id":"m2","chatID":"chat-1","text":"yo","timestamp":"2026-05-19T10:01:00Z","isSender":true,"senderName":"Me"}
  ],
  "hasMore": false, "oldestCursor": "o", "newestCursor": "n"
}`

func TestListMessages_MapsMessages(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(messagesJSON))
	})

	msgs, err := client.ListMessages(context.Background(), "chat-1", 50)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Text != "hey" {
		t.Errorf("msgs[0].Text = %q, want hey", msgs[0].Text)
	}
	if !msgs[1].IsFromMe {
		t.Error("msgs[1].IsFromMe = false, want true")
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/api/... -run TestListMessages`
Expected: build failure — `client.ListMessages undefined`.

- [ ] **Step 3: Implement**

Create `internal/api/messages.go`:

```go
package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// ListMessages fetches up to `limit` recent messages in a chat, oldest-first.
func (c *Client) ListMessages(ctx context.Context, chatID string, limit int) ([]Message, error) {
	page, err := c.sdk.Messages.List(ctx, chatID, beeperdesktopapi.MessageListParams{
		Limit: beeperdesktopapi.Int(int64(limit)),
	})
	if err != nil {
		return nil, fmt.Errorf("api: list messages for %s: %w", chatID, err)
	}
	out := make([]Message, 0, len(page.Items))
	for _, m := range page.Items {
		out = append(out, mapMessage(m))
	}
	return out, nil
}

func mapMessage(m beeperdesktopapi.shared.Message) Message {
	return Message{
		ID:         m.ID,
		ChatID:     m.ChatID,
		SenderName: m.SenderName,
		Text:       m.Text,
		Timestamp:  m.Timestamp,
		IsFromMe:   m.IsSender,
	}
}
```

**IMPORTANT for the implementer:** `Messages.List` returns messages in `shared.Message` (import `github.com/beeper/desktop-api-go/v5/shared`; the type reference `beeperdesktopapi.shared.Message` above is wrong Go syntax — import the `shared` package and use `shared.Message`). Verify field names with `go doc github.com/beeper/desktop-api-go/v5/shared.Message` — `Text`, `SenderName`, `Timestamp`, `IsSender`/`IsFromMe`, `ChatID` may differ. Adjust `mapMessage` and the import accordingly. Also confirm whether `MessageListParams.Limit` takes `param.Opt[int64]` via `beeperdesktopapi.Int`. The test JSON field names (`isSender`, `senderName`) are guesses — align them with the real `shared.Message` JSON tags after checking `go doc`, and re-capture from the live API if needed (`curl .../v1/chats/{id}/messages`).

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/api/... -run TestListMessages -v`
Expected: PASS after the field names are aligned.

- [ ] **Step 5: Commit**

```bash
git add internal/api/messages.go internal/api/messages_test.go
git commit -m "feat(api): add ListMessages"
```

---

## Task 8 — MarkRead

**Files:**
- Create: `internal/api/markread.go`
- Create: `internal/api/markread_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/api/markread_test.go`:

```go
package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestMarkRead_PostsToReadEndpoint(t *testing.T) {
	var gotPath, gotMethod string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat-1"}`))
	})

	err := client.MarkRead(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.Contains(gotPath, "/v1/chats/chat-1/read") {
		t.Errorf("path = %q, want it to contain /v1/chats/chat-1/read", gotPath)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/api/... -run TestMarkRead`
Expected: build failure — `client.MarkRead undefined`.

- [ ] **Step 3: Implement**

Create `internal/api/markread.go`:

```go
package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// MarkRead marks an entire chat as read.
func (c *Client) MarkRead(ctx context.Context, chatID string) error {
	_, err := c.sdk.Chats.MarkRead(ctx, chatID, beeperdesktopapi.ChatMarkReadParams{})
	if err != nil {
		return fmt.Errorf("api: mark read %s: %w", chatID, err)
	}
	return nil
}
```

**IMPORTANT for the implementer:** Confirm `ChatMarkReadParams` can be empty (the read endpoint may require a body field like a message ID or a boolean). Check `go doc github.com/beeper/desktop-api-go/v5.ChatMarkReadParams`. If a field is required, the spec's intent is "mark the whole chat read" — set whatever field marks-all-read (often omittable or a "mark read up to latest" flag). Adjust the params accordingly.

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/api/... -run TestMarkRead -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/markread.go internal/api/markread_test.go
git commit -m "feat(api): add MarkRead"
```

---

## Task 9 — Integration test against the real local API (build-tagged)

**Files:**
- Create: `internal/api/integration_test.go`

- [ ] **Step 1: Write the build-tagged integration test**

Create `internal/api/integration_test.go`:

```go
//go:build integration

package api_test

import (
	"context"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

// Run with: go test -tags=integration ./internal/api/...
// Requires Beeper Desktop running with the Desktop API enabled and
// BEEPER_ACCESS_TOKEN set in the environment.
func TestIntegration_ListChats(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Token == "" {
		t.Skip("BEEPER_ACCESS_TOKEN not set; skipping integration test")
	}

	client := api.New(cfg)
	chats, err := client.ListChats(context.Background())
	if err != nil {
		t.Fatalf("ListChats() against real API error = %v", err)
	}
	if len(chats) == 0 {
		t.Fatal("ListChats() returned 0 chats from real API; expected at least one")
	}
	t.Logf("fetched %d chats from real Beeper Desktop", len(chats))
}
```

- [ ] **Step 2: Confirm it's excluded from the normal suite**

Run: `go test ./internal/api/...`
Expected: the integration test does NOT run (no `integration` tag); other api tests pass.

- [ ] **Step 3: Run it explicitly against the real API**

Run: `go test -tags=integration ./internal/api/... -run TestIntegration_ListChats -v`
Expected: PASS, logging a chat count > 0. (Beeper Desktop must be running and `BEEPER_ACCESS_TOKEN` set.) If it fails with an auth error, the token isn't being read — confirm the env var is exported in the shell running the test.

- [ ] **Step 4: Commit**

```bash
git add internal/api/integration_test.go
git commit -m "test(api): add build-tagged integration test against real API"
```

---

## Task 10 — Wire ListChats into main.go (visible result)

This is the payoff: the binary prints your real chats instead of placeholder status.

**Files:**
- Modify: `cmd/beeper-tui/main.go`

- [ ] **Step 1: Update main.go to fetch and print chats**

Replace the cache-status block at the end of `main()` so that, after loading config and cache, it fetches and prints the live chat list. The full new `cmd/beeper-tui/main.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/state"
)

const version = "0.0.0-phase-2"

func main() {
	fmt.Printf("beeper-tui %s\n", version)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "No BEEPER_ACCESS_TOKEN set. Enable the Desktop API in Beeper (Settings -> Developers -> Approved connections) and export a token.")
		os.Exit(1)
	}

	cachePath := filepath.Join(cfg.CacheDir, "cache.json")
	if cache, err := state.Load(cachePath); err == nil {
		fmt.Printf("  cache: %d chats (warm)\n", len(cache.Chats))
	} else if !errors.Is(err, state.ErrCorruptCache) && !errors.Is(err, state.ErrSchemaMismatch) {
		fmt.Fprintf(os.Stderr, "cache: %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := api.New(cfg)
	chats, err := client.ListChats(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch chats: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n%d chats:\n", len(chats))
	for _, ch := range chats {
		marker := " "
		if ch.Unread > 0 {
			marker = "*"
		}
		fmt.Printf("  %s [%-10s] %3d  %s\n", marker, ch.Network, ch.Unread, ch.Title)
	}
}
```

- [ ] **Step 2: Build and run against the real API**

Run:
```bash
go build ./cmd/beeper-tui
./beeper-tui
```
Expected: prints the version, a cache line, then your real chat list — one line per chat with an unread marker, network, unread count, and title. (Requires Beeper Desktop running and `BEEPER_ACCESS_TOKEN` exported in the shell.)

- [ ] **Step 3: Verify the no-token path still degrades gracefully**

Run: `env -u BEEPER_ACCESS_TOKEN ./beeper-tui`
Expected: prints the "No BEEPER_ACCESS_TOKEN set" guidance and exits non-zero, without a stack trace.

- [ ] **Step 4: Confirm the unit suite still passes**

Run: `go test ./...`
Expected: all packages pass (config, state, api). cmd has no tests.

- [ ] **Step 5: Commit**

```bash
git add cmd/beeper-tui/main.go
git commit -m "feat(main): fetch and print real chat list via api client"
```

---

## Task 11 — Close bd T1 and tag

- [ ] **Step 1: Close the bd issue**

```bash
bd close beeper-tui-qbg.4 --reason "REST client complete: api.New, ListChats (paginated), ListMessages, MarkRead, domain types, httptest unit tests + build-tagged integration test. Wired into main.go which now prints the real chat list."
```

- [ ] **Step 2: Sanity check + tag**

```bash
go test ./... && go vet ./... && git tag -a phase-2-complete -m "Phase 2: REST client (ListChats/ListMessages/MarkRead) wired into main"
```
Expected: tests pass, vet clean, tag created.

- [ ] **Step 3: Check what's unblocked**

```bash
bd ready
```
Expected: U1 (Model + reducer) and U2 (View) are the natural next work for Phase 3.

---

## Self-Review notes (for the controller)

- **SDK signature risk is the main hazard.** Tasks 1, 4, 5, 7, 8 each carry an "IMPORTANT for the implementer" note flagging exact field/method names to verify with `go doc`. The first implementer to touch the SDK (Task 1, 4) should report the real `ChatListResponse` / `shared.Message` struct shapes back so the controller can correct later tasks before dispatching them.
- **Test-JSON fidelity:** the canned JSON in tests was hand-derived from one live capture. If the SDK's parser rejects it (strict field types), re-capture a real response body with `curl` and trim it into the fixture.
- **Spec coverage:** T1 in the spec asked for ListChats, ListMessages, MarkRead — all covered (Tasks 4-8). Pagination (Task 5) and error handling (Task 6) are covered. Integration test (Task 9) matches the spec's testing section. WS is correctly absent (deferred).
- **No placeholders** in test/impl code, but several mapping functions explicitly depend on `go doc` verification — this is honest given we cannot compile against the SDK while writing the plan. The integration test (Task 9) and the real `main.go` run (Task 10) are the backstop that proves the mappings are actually correct against live data.
