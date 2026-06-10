package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/state"
)

// cacheSaveDebounce bounds how often background refreshes rewrite the cache
// file. The final write on quit captures whatever the debounce skipped.
const cacheSaveDebounce = 10 * time.Second

// WithCache warm-starts the model from a cached snapshot, so the first frame
// shows chats instead of "Loading…". Init's REST fetch still runs and
// replaces the snapshot when fresh data lands.
func (m Model) WithCache(c state.Cache, path string) Model {
	m.cachePath = path
	if len(c.Chats) == 0 {
		return m
	}
	sortChats(c.Chats)
	m.chats = c.Chats
	m.selected = reselectByID(c.Chats, c.LastSelectedChatID)
	m.loadingChats = false
	return m
}

// Snapshot captures what the next launch needs to warm-start.
func (m Model) Snapshot() state.Cache {
	var selectedID string
	if m.selected < len(m.chats) {
		selectedID = m.chats[m.selected].ID
	}
	return state.Cache{LastSelectedChatID: selectedID, Chats: m.chats}
}

// maybeSaveCache returns a debounced background write of the current snapshot.
// Saves are best-effort: a failed write never surfaces, because the next
// refresh or the write on quit retries it.
func (m Model) maybeSaveCache() (Model, tea.Cmd) {
	if m.cachePath == "" || len(m.chats) == 0 || time.Since(m.cacheSavedAt) < cacheSaveDebounce {
		return m, nil
	}
	m.cacheSavedAt = time.Now()
	path := m.cachePath
	snap := m.Snapshot()
	return m, func() tea.Msg {
		_ = state.Save(path, snap)
		return nil
	}
}
