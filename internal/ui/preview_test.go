package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
)

func previewTestModel() Model {
	return Model{
		mode:   ModeList,
		chats:  []api.Chat{{ID: "c1", Title: "Alpha"}, {ID: "c2", Title: "Beta"}},
		width:  80,
		height: 24,
	}
}

func TestTogglePreview_OnFiresLoad(t *testing.T) {
	m := previewTestModel()
	m, cmd := m.togglePreview()
	if !m.previewOn {
		t.Fatal("previewOn = false after toggle, want true")
	}
	if cmd == nil {
		t.Error("togglePreview returned nil cmd, want a preview load")
	}
	m, cmd = m.togglePreview()
	if m.previewOn {
		t.Error("previewOn = true after second toggle, want false")
	}
	if cmd != nil {
		t.Error("togglePreview off returned a cmd, want nil")
	}
}

func TestTogglePreview_IgnoredOutsideList(t *testing.T) {
	m := previewTestModel()
	m.mode = ModeConversation
	m, cmd := m.togglePreview()
	if m.previewOn || cmd != nil {
		t.Error("togglePreview in conversation mode should be a no-op")
	}
}

func TestPreviewLoad_SkipsCachedChat(t *testing.T) {
	m := previewTestModel()
	m.previewOn = true
	m.previewCache = map[string][]api.Message{"c1": {}}
	if cmd := m.previewLoad(); cmd != nil {
		t.Error("previewLoad returned a cmd for a cached chat, want nil")
	}
	m.selected = 1
	if cmd := m.previewLoad(); cmd == nil {
		t.Error("previewLoad returned nil for an uncached chat, want a cmd")
	}
}

func TestApplyPreviewLoaded_CachesAndClearsError(t *testing.T) {
	m := previewTestModel()
	m = m.applyPreviewLoaded(previewLoadedMsg{chatID: "c1", err: errors.New("boom")})
	if m.previewErr["c1"] == nil {
		t.Fatal("error not recorded")
	}
	msgs := []api.Message{{ID: "m1", ChatID: "c1", SenderName: "Ann", Text: "hi"}}
	m = m.applyPreviewLoaded(previewLoadedMsg{chatID: "c1", messages: msgs})
	if len(m.previewCache["c1"]) != 1 {
		t.Error("messages not cached")
	}
	if m.previewErr["c1"] != nil {
		t.Error("stale error not cleared after successful load")
	}
}

func TestRenderList_PreviewShowsSelectedChatMessages(t *testing.T) {
	m := previewTestModel()
	m.previewOn = true
	m.previewCache = map[string][]api.Message{
		"c1": {{ID: "m1", ChatID: "c1", SenderName: "Ann", Text: "hello world"}},
	}
	out := m.renderList()
	if !strings.Contains(out, "hello world") {
		t.Errorf("preview pane missing message text:\n%s", out)
	}
	if !strings.Contains(out, "│") {
		t.Errorf("preview pane separator missing:\n%s", out)
	}
}

func TestRenderList_PreviewLoadingPlaceholder(t *testing.T) {
	m := previewTestModel()
	m.previewOn = true
	out := m.renderList()
	if !strings.Contains(out, "loading…") {
		t.Errorf("preview pane missing loading placeholder:\n%s", out)
	}
}

func TestJoinPreview_NarrowTerminalFallsBack(t *testing.T) {
	m := previewTestModel()
	m.previewOn = true
	m.width = 30
	rows := []string{"row1", "row2"}
	out := m.joinPreview(rows)
	if len(out) != 2 || out[0] != "row1" {
		t.Errorf("narrow terminal should return rows unchanged, got %v", out)
	}
}
