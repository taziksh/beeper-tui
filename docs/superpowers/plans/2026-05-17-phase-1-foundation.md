# Phase 1 — Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundation packages for beeper-tui: a buildable Go module skeleton, a `config` package that resolves XDG paths and discovers the Beeper access token from the environment, and a `state` package that persistently caches chat-list snapshots to disk.

**Architecture:** Standard Go module layout (`cmd/beeper-tui/` for the entry point, `internal/` for everything else). `config` and `state` are pure data layers with one IO boundary each (env vars; file load/save). No bubbletea, no Beeper SDK, no networking yet — those land in Phases 2–3. Test-driven throughout: every behavior is locked in by a failing unit test before implementation.

**Tech Stack:**
- Go 1.22+ (uses `t.Setenv`, recent stdlib slices/maps idioms)
- Standard library only for Phase 1 (no third-party deps — `encoding/json`, `os`, `testing`, `errors`, `path/filepath`)
- bd CLI for issue tracking (already installed; database initialized at project root)

**bd issues covered by this plan:**
- `beeper-tui-qbg.1` (F1: Project scaffolding)
- `beeper-tui-qbg.2` (F2: Config layer)
- `beeper-tui-qbg.3` (F3: Cache layer)

**Module path:** `github.com/taziksh/beeper-tui`

---

## File Structure

| File | Created in task | Responsibility |
|---|---|---|
| `go.mod` | 1 | Module declaration, Go version |
| `LICENSE` | 3 | MIT license text |
| `README.md` | 4 | Install/usage skeleton |
| `cmd/beeper-tui/main.go` | 2 | Entry point: flag parsing, config load, cache load, prints status |
| `internal/config/config.go` | 6–10 | XDG path resolution, token discovery, base URL discovery, `Config` struct, `Load()` |
| `internal/config/config_test.go` | 6–10 | Unit tests for every config function |
| `internal/state/cache.go` | 13–17 | `Cache` struct, `Save()`, `Load()`, sentinel errors, schema version |
| `internal/state/cache_test.go` | 13–17 | Unit tests covering happy path, missing file, corrupt JSON, schema mismatch |

`internal/api/`, `internal/ws/`, `internal/ui/` are deliberately **not** created in Phase 1 — Go will complain about empty packages and they aren't yet needed. They appear in Phase 2 onward.

---

## Conventions

**Commit messages:** Use Conventional Commits (`feat:`, `fix:`, `test:`, `chore:`, `docs:`). Each Task ends in a single commit unless noted.

**Test names:** `Test<Function>_<Scenario>` (e.g., `TestToken_ReadsEnvVar`, `TestLoad_MissingFileReturnsEmpty`).

**Error sentinels:** Use `errors.New("package: short description")` for package-level sentinel errors so callers can `errors.Is()` them.

**No comments unless they explain WHY.** The code should be self-documenting; comments rot.

---

## Task 1 — Initialize Go module

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Initialize module**

Run:
```bash
cd /Users/tazik/Projects/beeper-tui
go mod init github.com/taziksh/beeper-tui
```

Expected output:
```
go: creating new go.mod: module github.com/taziksh/beeper-tui
```

- [ ] **Step 2: Verify go.mod**

Run: `cat go.mod`

Expected content (Go toolchain line will reflect your installed version):
```
module github.com/taziksh/beeper-tui

go 1.22
```

If your installed Go is newer (1.23+, 1.24+), the version line will reflect that — that's fine. If it's older than 1.22, install a newer Go toolchain before continuing.

- [ ] **Step 3: Commit**

```bash
git add go.mod
git commit -m "chore: initialize Go module"
```

---

## Task 2 — main.go entry point

**Files:**
- Create: `cmd/beeper-tui/main.go`

- [ ] **Step 1: Create directory**

Run: `mkdir -p cmd/beeper-tui`

- [ ] **Step 2: Write main.go**

Create `cmd/beeper-tui/main.go`:

```go
package main

import "fmt"

const version = "0.0.0-phase-1"

func main() {
	fmt.Printf("beeper-tui %s\n", version)
}
```

- [ ] **Step 3: Build and run**

Run:
```bash
go build ./cmd/beeper-tui
./beeper-tui
```

Expected output:
```
beeper-tui 0.0.0-phase-1
```

- [ ] **Step 4: Add binary to .gitignore**

Append to `.gitignore`:
```

# Built binary (use `go install` or `go build -o` for distribution)
/beeper-tui
```

- [ ] **Step 5: Commit**

```bash
git add cmd/beeper-tui/main.go .gitignore
git commit -m "chore: add main.go skeleton that prints version"
```

---

## Task 3 — Add MIT LICENSE

**Files:**
- Create: `LICENSE`

- [ ] **Step 1: Write LICENSE**

Create `LICENSE`:

```
MIT License

Copyright (c) 2026 Tazik Shahjahan

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 2: Commit**

```bash
git add LICENSE
git commit -m "chore: add MIT license"
```

---

## Task 4 — README skeleton

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README**

Create `README.md`:

````markdown
# beeper-tui

A keyboard-driven terminal UI for [Beeper](https://beeper.com), built on top of the local Beeper Desktop API.

> **Status:** under construction. v1 (read-only triage) is in progress. See [the v1 design spec](docs/superpowers/specs/2026-05-17-beeper-tui-design.md).

## Requirements

- Beeper Desktop running locally with the Developer API enabled (Settings → Developers → Beeper Desktop API). Requires Beeper Desktop v4.1.169+.
- Go 1.22 or later (for `go install`).

## Install (development)

```bash
go install github.com/taziksh/beeper-tui/cmd/beeper-tui@latest
```

## Configuration

The TUI auto-discovers your access token from a locally-running Beeper Desktop.

For headless use, set the token explicitly:

```bash
export BEEPER_ACCESS_TOKEN=<token>
```

To override the API base URL (rare):

```bash
export BEEPER_API_BASE_URL=http://localhost:23373
```

## Roadmap

- **v1** — read-only triage (chat list, debounced preview, reading mode)
- **v1.1** — search across chats and messages
- **v2** — send text messages
- **v3** — attachments, reactions, replies, threads, edits, deletes

## License

MIT. See [LICENSE](LICENSE).
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README skeleton"
```

---

## Task 5 — Close bd issue F1 (scaffolding)

- [ ] **Step 1: Close the issue**

Run:
```bash
bd close beeper-tui-qbg.1 --reason "Scaffolding complete: Go module, main.go stub, LICENSE, README. See commits for details."
```

Expected: closure confirmation.

- [ ] **Step 2: Verify it's closed**

Run: `bd show beeper-tui-qbg.1 | head -10`

Expected: status shows `closed`.

---

## Task 6 — config.XDGConfigDir() — failing test first

**Files:**
- Create: `internal/config/config_test.go`
- Create: `internal/config/config.go`

- [ ] **Step 1: Create the test file with a failing test**

Create `internal/config/config_test.go`:

```go
package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/config"
)

func TestXDGConfigDir_EndsInBeeperTUI(t *testing.T) {
	got, err := config.XDGConfigDir()
	if err != nil {
		t.Fatalf("XDGConfigDir() error = %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("beeper-tui")) {
		t.Errorf("XDGConfigDir() = %q, want path ending in 'beeper-tui'", got)
	}
}
```

- [ ] **Step 2: Run the test — confirm it fails to compile**

Run: `go test ./internal/config/...`

Expected: build failure mentioning `config.XDGConfigDir undefined` (because we haven't written the function).

- [ ] **Step 3: Write the minimal implementation**

Create `internal/config/config.go`:

```go
package config

import (
	"os"
	"path/filepath"
)

const appName = "beeper-tui"

func XDGConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}
```

- [ ] **Step 4: Run the test — confirm it passes**

Run: `go test ./internal/config/... -v`

Expected: `--- PASS: TestXDGConfigDir_EndsInBeeperTUI`

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add XDGConfigDir resolver"
```

---

## Task 7 — config.XDGCacheDir()

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/config/config_test.go`:

```go
func TestXDGCacheDir_EndsInBeeperTUI(t *testing.T) {
	got, err := config.XDGCacheDir()
	if err != nil {
		t.Fatalf("XDGCacheDir() error = %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("beeper-tui")) {
		t.Errorf("XDGCacheDir() = %q, want path ending in 'beeper-tui'", got)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/config/... -run TestXDGCacheDir`

Expected: `config.XDGCacheDir undefined`.

- [ ] **Step 3: Implement**

Append to `internal/config/config.go`:

```go
func XDGCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/config/... -v`

Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add XDGCacheDir resolver"
```

---

## Task 8 — config.Token() reads BEEPER_ACCESS_TOKEN

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add the failing test (table-driven)**

Append to `internal/config/config_test.go`:

```go
func TestToken_ReadsEnvVar(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"set", "abc123", "abc123"},
		{"empty", "", ""},
		{"whitespace stripped", "  xyz  ", "xyz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BEEPER_ACCESS_TOKEN", tc.env)
			if got := config.Token(); got != tc.want {
				t.Errorf("Token() = %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/config/... -run TestToken`

Expected: `config.Token undefined`.

- [ ] **Step 3: Implement**

Append to `internal/config/config.go`:

```go
import (
	"os"
	"path/filepath"
	"strings"
)
```

(If `strings` is already imported via the existing imports, just ensure it's present.)

Then add:

```go
func Token() string {
	return strings.TrimSpace(os.Getenv("BEEPER_ACCESS_TOKEN"))
}
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/config/... -v`

Expected: all three subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add Token() reading BEEPER_ACCESS_TOKEN"
```

---

## Task 9 — config.BaseURL() reads BEEPER_API_BASE_URL with default

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/config/config_test.go`:

```go
func TestBaseURL_DefaultsToLocalhost(t *testing.T) {
	t.Setenv("BEEPER_API_BASE_URL", "")
	if got, want := config.BaseURL(), "http://127.0.0.1:23373"; got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}

func TestBaseURL_HonorsEnvOverride(t *testing.T) {
	t.Setenv("BEEPER_API_BASE_URL", "http://192.168.1.10:9999")
	if got, want := config.BaseURL(), "http://192.168.1.10:9999"; got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/config/... -run TestBaseURL`

Expected: `config.BaseURL undefined`.

- [ ] **Step 3: Implement**

Append to `internal/config/config.go`:

```go
const defaultBaseURL = "http://127.0.0.1:23373"

func BaseURL() string {
	if v := strings.TrimSpace(os.Getenv("BEEPER_API_BASE_URL")); v != "" {
		return v
	}
	return defaultBaseURL
}
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/config/... -v`

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add BaseURL() with env override"
```

---

## Task 10 — config.Load() bundles everything into a Config struct

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/config/config_test.go`:

```go
func TestLoad_AssemblesAllFields(t *testing.T) {
	t.Setenv("BEEPER_ACCESS_TOKEN", "tok")
	t.Setenv("BEEPER_API_BASE_URL", "http://x.test")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Token != "tok" {
		t.Errorf("Token = %q, want %q", got.Token, "tok")
	}
	if got.BaseURL != "http://x.test" {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, "http://x.test")
	}
	if got.ConfigDir == "" {
		t.Error("ConfigDir is empty")
	}
	if got.CacheDir == "" {
		t.Error("CacheDir is empty")
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/config/... -run TestLoad`

Expected: `config.Load undefined` / `config.Config undefined`.

- [ ] **Step 3: Implement**

Append to `internal/config/config.go`:

```go
type Config struct {
	Token     string
	BaseURL   string
	ConfigDir string
	CacheDir  string
}

func Load() (Config, error) {
	cfgDir, err := XDGConfigDir()
	if err != nil {
		return Config{}, err
	}
	cacheDir, err := XDGCacheDir()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Token:     Token(),
		BaseURL:   BaseURL(),
		ConfigDir: cfgDir,
		CacheDir:  cacheDir,
	}, nil
}
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/config/... -v`

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add Config struct and Load()"
```

---

## Task 11 — Wire config.Load() into main.go (smoke test)

**Files:**
- Modify: `cmd/beeper-tui/main.go`

- [ ] **Step 1: Rewrite main.go to load and print config status**

Replace `cmd/beeper-tui/main.go` entirely with:

```go
package main

import (
	"fmt"
	"os"

	"github.com/taziksh/beeper-tui/internal/config"
)

const version = "0.0.0-phase-1"

func main() {
	fmt.Printf("beeper-tui %s\n", version)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	tokenStatus := "not set"
	if cfg.Token != "" {
		tokenStatus = "set"
	}

	fmt.Printf("  api base url: %s\n", cfg.BaseURL)
	fmt.Printf("  config dir:   %s\n", cfg.ConfigDir)
	fmt.Printf("  cache dir:    %s\n", cfg.CacheDir)
	fmt.Printf("  token:        %s\n", tokenStatus)
}
```

- [ ] **Step 2: Build and run — manual smoke**

Run:
```bash
go build ./cmd/beeper-tui
./beeper-tui
```

Expected output (paths will reflect your OS):
```
beeper-tui 0.0.0-phase-1
  api base url: http://127.0.0.1:23373
  config dir:   /Users/tazik/Library/Application Support/beeper-tui
  cache dir:    /Users/tazik/Library/Caches/beeper-tui
  token:        not set
```

- [ ] **Step 3: Smoke test with token set**

Run:
```bash
BEEPER_ACCESS_TOKEN=test123 ./beeper-tui
```

Expected: `token: set` on the last line.

- [ ] **Step 4: Commit**

```bash
git add cmd/beeper-tui/main.go
git commit -m "feat(main): load and print config on startup"
```

---

## Task 12 — Close bd issue F2 (config layer)

- [ ] **Step 1: Close the issue**

Run:
```bash
bd close beeper-tui-qbg.2 --notes "Config layer complete: XDGConfigDir, XDGCacheDir, Token, BaseURL, Load(). Wired into main.go. config.toml support deferred — env vars (BEEPER_ACCESS_TOKEN, BEEPER_API_BASE_URL) cover the same surface for v1 and can be revisited if real user demand surfaces. All tests passing."
```

- [ ] **Step 2: Verify**

Run: `bd show beeper-tui-qbg.2 | head -10`

Expected: status `closed`.

---

## Task 13 — state.Cache struct + Save() (happy path)

**Files:**
- Create: `internal/state/cache_test.go`
- Create: `internal/state/cache.go`

- [ ] **Step 1: Create the test file with a failing Save test**

Create `internal/state/cache_test.go`:

```go
package state_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/state"
)

func TestSave_WritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	c := state.Cache{
		SchemaVersion: state.CurrentSchemaVersion,
		LastSelectedChatID: "chat-123",
		Chats: []state.ChatSnapshot{
			{
				ID:        "chat-123",
				Name:      "Sarah Kim",
				Account:   "iMessage",
				Unread:    3,
				LastTs:    time.Date(2026, 5, 17, 10, 42, 0, 0, time.UTC),
				LastBody:  "hey did you see the article",
			},
		},
	}

	if err := state.Save(path, c); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var decoded state.Cache
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, raw: %s", err, raw)
	}
	if decoded.LastSelectedChatID != "chat-123" {
		t.Errorf("LastSelectedChatID = %q, want %q", decoded.LastSelectedChatID, "chat-123")
	}
	if len(decoded.Chats) != 1 {
		t.Fatalf("len(Chats) = %d, want 1", len(decoded.Chats))
	}
	if decoded.Chats[0].Name != "Sarah Kim" {
		t.Errorf("Chats[0].Name = %q, want %q", decoded.Chats[0].Name, "Sarah Kim")
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/state/...`

Expected: build failure (`state.Cache undefined`, etc.).

- [ ] **Step 3: Implement Cache + Save**

Create `internal/state/cache.go`:

```go
package state

import (
	"encoding/json"
	"os"
	"time"
)

const CurrentSchemaVersion = 1

type Cache struct {
	SchemaVersion      int            `json:"schema_version"`
	LastSelectedChatID string         `json:"last_selected_chat_id"`
	Chats              []ChatSnapshot `json:"chats"`
}

type ChatSnapshot struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Account  string    `json:"account"`
	Unread   int       `json:"unread"`
	LastTs   time.Time `json:"last_ts"`
	LastBody string    `json:"last_body"`
}

func Save(path string, c Cache) error {
	c.SchemaVersion = CurrentSchemaVersion
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/state/... -v`

Expected: `--- PASS: TestSave_WritesValidJSON`.

- [ ] **Step 5: Commit**

```bash
git add internal/state/
git commit -m "feat(state): add Cache struct and Save()"
```

---

## Task 14 — state.Load() round-trips a Save()

**Files:**
- Modify: `internal/state/cache_test.go`
- Modify: `internal/state/cache.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/state/cache_test.go`:

```go
func TestLoad_RoundTripsSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	original := state.Cache{
		LastSelectedChatID: "abc",
		Chats: []state.ChatSnapshot{
			{ID: "abc", Name: "Test Chat", Account: "Signal", Unread: 1},
		},
	}
	if err := state.Save(path, original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.LastSelectedChatID != "abc" {
		t.Errorf("LastSelectedChatID = %q, want %q", got.LastSelectedChatID, "abc")
	}
	if len(got.Chats) != 1 || got.Chats[0].Name != "Test Chat" {
		t.Errorf("Chats = %+v, want one chat named Test Chat", got.Chats)
	}
	if got.SchemaVersion != state.CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", got.SchemaVersion, state.CurrentSchemaVersion)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/state/... -run TestLoad_RoundTrips`

Expected: `state.Load undefined`.

- [ ] **Step 3: Implement Load**

Append to `internal/state/cache.go`:

```go
func Load(path string) (Cache, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Cache{}, err
	}
	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cache{}, err
	}
	return c, nil
}
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/state/... -v`

Expected: both Save and Load tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/state/
git commit -m "feat(state): add Load() that round-trips Save()"
```

---

## Task 15 — Load() returns empty cache for a missing file (no error)

This encodes the spec rule: "Cache is purely an optimization. If the cache is missing or unreadable, cold-load gracefully."

**Files:**
- Modify: `internal/state/cache_test.go`
- Modify: `internal/state/cache.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/state/cache_test.go`:

```go
func TestLoad_MissingFileReturnsEmptyCacheNoError(t *testing.T) {
	got, err := state.Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for missing file", err)
	}
	if len(got.Chats) != 0 {
		t.Errorf("Chats = %+v, want empty", got.Chats)
	}
	if got.SchemaVersion != 0 {
		t.Errorf("SchemaVersion = %d, want 0 for missing file", got.SchemaVersion)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/state/... -run TestLoad_MissingFile`

Expected: FAIL — current `Load()` returns an `*os.PathError`.

- [ ] **Step 3: Update Load to swallow file-not-found**

Modify `Load()` in `internal/state/cache.go`:

```go
func Load(path string) (Cache, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Cache{}, nil
		}
		return Cache{}, err
	}
	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cache{}, err
	}
	return c, nil
}
```

Add `"errors"` to the import block.

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/state/... -v`

Expected: all three tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/state/
git commit -m "feat(state): Load returns empty cache for missing file"
```

---

## Task 16 — Load() returns ErrCorruptCache sentinel for invalid JSON

**Files:**
- Modify: `internal/state/cache_test.go`
- Modify: `internal/state/cache.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/state/cache_test.go`:

```go
import (
	// ... existing imports ...
	"errors"
)
```

(Add `errors` to the import block if not already there.)

```go
func TestLoad_CorruptJSONReturnsSentinel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o600); err != nil {
		t.Fatalf("setup WriteFile() error = %v", err)
	}

	_, err := state.Load(path)
	if !errors.Is(err, state.ErrCorruptCache) {
		t.Errorf("Load() error = %v, want errors.Is(err, ErrCorruptCache)", err)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/state/... -run TestLoad_Corrupt`

Expected: FAIL — `state.ErrCorruptCache` is undefined.

- [ ] **Step 3: Add the sentinel and wrap the JSON error**

Modify `internal/state/cache.go`:

```go
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

const CurrentSchemaVersion = 1

var ErrCorruptCache = errors.New("state: cache file is corrupt")
```

Update `Load()`:

```go
func Load(path string) (Cache, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Cache{}, nil
		}
		return Cache{}, err
	}
	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cache{}, fmt.Errorf("%w: %v", ErrCorruptCache, err)
	}
	return c, nil
}
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/state/... -v`

Expected: all four tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/state/
git commit -m "feat(state): Load returns ErrCorruptCache for invalid JSON"
```

---

## Task 17 — Load() returns ErrSchemaMismatch when version differs

**Files:**
- Modify: `internal/state/cache_test.go`
- Modify: `internal/state/cache.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/state/cache_test.go`:

```go
func TestLoad_SchemaMismatchReturnsSentinel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	// Write a valid JSON cache with a future schema version.
	body := []byte(`{"schema_version": 999, "last_selected_chat_id": "x", "chats": []}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("setup WriteFile() error = %v", err)
	}

	_, err := state.Load(path)
	if !errors.Is(err, state.ErrSchemaMismatch) {
		t.Errorf("Load() error = %v, want errors.Is(err, ErrSchemaMismatch)", err)
	}
}
```

- [ ] **Step 2: Run — confirm failure**

Run: `go test ./internal/state/... -run TestLoad_SchemaMismatch`

Expected: FAIL — `state.ErrSchemaMismatch` is undefined.

- [ ] **Step 3: Implement**

Add to `internal/state/cache.go` (near `ErrCorruptCache`):

```go
var ErrSchemaMismatch = errors.New("state: cache schema version does not match current")
```

Update `Load()` to check schema version after unmarshal:

```go
func Load(path string) (Cache, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Cache{}, nil
		}
		return Cache{}, err
	}
	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cache{}, fmt.Errorf("%w: %v", ErrCorruptCache, err)
	}
	if c.SchemaVersion != CurrentSchemaVersion {
		return Cache{}, fmt.Errorf("%w: file has %d, expected %d",
			ErrSchemaMismatch, c.SchemaVersion, CurrentSchemaVersion)
	}
	return c, nil
}
```

- [ ] **Step 4: Run — confirm pass**

Run: `go test ./internal/state/... -v`

Expected: all five tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/state/
git commit -m "feat(state): Load returns ErrSchemaMismatch for stale schema"
```

---

## Task 18 — Wire cache load + save into main.go (smoke test)

**Files:**
- Modify: `cmd/beeper-tui/main.go`

- [ ] **Step 1: Update main.go to load + save the cache**

Replace `cmd/beeper-tui/main.go` entirely:

```go
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/state"
)

const version = "0.0.0-phase-1"

func main() {
	fmt.Printf("beeper-tui %s\n", version)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	tokenStatus := "not set"
	if cfg.Token != "" {
		tokenStatus = "set"
	}

	fmt.Printf("  api base url: %s\n", cfg.BaseURL)
	fmt.Printf("  config dir:   %s\n", cfg.ConfigDir)
	fmt.Printf("  cache dir:    %s\n", cfg.CacheDir)
	fmt.Printf("  token:        %s\n", tokenStatus)

	cachePath := filepath.Join(cfg.CacheDir, "cache.json")
	cache, err := state.Load(cachePath)
	switch {
	case err == nil:
		fmt.Printf("  cache:        loaded %d chats\n", len(cache.Chats))
	case errors.Is(err, state.ErrCorruptCache):
		fmt.Printf("  cache:        corrupt, starting fresh\n")
	case errors.Is(err, state.ErrSchemaMismatch):
		fmt.Printf("  cache:        schema mismatch, starting fresh\n")
	default:
		fmt.Fprintf(os.Stderr, "cache: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build and run — manual smoke**

Run:
```bash
go build ./cmd/beeper-tui
./beeper-tui
```

Expected: the existing output plus a line like `cache:        loaded 0 chats` (the first time, the cache dir may not exist — that's fine, you'll see "loaded 0 chats" because `os.ErrNotExist` returns an empty Cache).

- [ ] **Step 3: Verify corrupt-cache handling manually**

Run (mac):
```bash
mkdir -p "$HOME/Library/Caches/beeper-tui"
echo "junk" > "$HOME/Library/Caches/beeper-tui/cache.json"
./beeper-tui
```

(Linux equivalent: `$HOME/.cache/beeper-tui/cache.json`.)

Expected: `cache:        corrupt, starting fresh`.

Clean up:
```bash
rm "$HOME/Library/Caches/beeper-tui/cache.json"  # or the Linux path
```

- [ ] **Step 4: Commit**

```bash
git add cmd/beeper-tui/main.go
git commit -m "feat(main): load cache and report status on startup"
```

---

## Task 19 — Close bd issue F3 (cache layer)

- [ ] **Step 1: Close the issue**

Run:
```bash
bd close beeper-tui-qbg.3 --reason "Cache layer complete: Cache struct, Save, Load with sentinels for missing/corrupt/schema-mismatch. Wired into main.go."
```

- [ ] **Step 2: Verify**

Run: `bd show beeper-tui-qbg.3 | head -10`

Expected: status `closed`.

---

## Task 20 — Phase 1 sanity check

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`

Expected: all tests across `internal/config` and `internal/state` PASS. Nothing else has tests yet.

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`

Expected: no output (vet found no issues).

- [ ] **Step 3: Verify binary builds clean**

Run:
```bash
go build ./cmd/beeper-tui
./beeper-tui
```

Expected: prints version, config status, and cache status without errors.

- [ ] **Step 4: Check what's ready next per bd**

Run: `bd ready`

Expected: F2 and F3 are closed, so the dependency graph should now show **T1 (REST client), T2 (WebSocket client), U1 (Model + reducer)** as the newly-ready issues. F1 was already done.

- [ ] **Step 5: Tag the milestone (optional)**

```bash
git tag -a phase-1-complete -m "Phase 1: scaffolding, config, cache layers done"
```

This makes Phase 1 a stable reference point if we ever want to bisect or roll back.

---

## What's next

When Phase 1 ships, Phase 2 covers the **transport layer** (T1: REST client over the Beeper Go SDK, T2: WebSocket client with reconnect). These can be developed in parallel — they share no code. After Phase 2, Phase 3 brings up the UI core (U1: bubbletea Model + reducer, U2: lipgloss View). Then Phase 4 wires features (X1–X8). Then Phase 5 lands the tests and v1 release.

Each phase gets its own plan written when the previous phase is complete and we know what we actually have on disk.
