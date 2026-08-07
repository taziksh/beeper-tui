package ui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestRenderHalfBlocks_HalvesHeightAndCapsWidth(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 200, A: 0xff})
		}
	}
	lines := renderHalfBlocks(img, 8, 10)
	if len(lines) != 4 {
		t.Fatalf("got %d rows, want 4 (8px tall image at two pixels per row)", len(lines))
	}
	if !strings.Contains(lines[0], "▀") {
		t.Errorf("rows should be drawn with half blocks, got %q", lines[0])
	}
}

func TestRenderHalfBlocks_TallImageRespectsMaxRows(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 100))
	lines := renderHalfBlocks(img, 40, 9)
	if len(lines) != 9 {
		t.Errorf("got %d rows, want the 9-row cap", len(lines))
	}
}

func TestPastedFilePath_RecognizesDraggedMediaFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, paste := range []string{path, path + " ", "'" + path + "'"} {
		if got := pastedFilePath(paste); got != path {
			t.Errorf("pastedFilePath(%q) = %q, want %q", paste, got, path)
		}
	}
}

func TestPastedFilePath_RejectsOrdinaryText(t *testing.T) {
	for _, paste := range []string{"hello there", "/no/such/file.png", "see /tmp for details"} {
		if got := pastedFilePath(paste); got != "" {
			t.Errorf("pastedFilePath(%q) = %q, want empty", paste, got)
		}
	}
}

func TestMessageBlock_ImageAttachmentShowsLoadingThenPreview(t *testing.T) {
	att := api.Attachment{Type: "img", ID: "mxc://x/1", FileName: "pic.jpg", FileSize: 2 << 20}
	m := Model{
		mode:     ModeConversation,
		messages: []api.Message{{ID: "m1", SenderName: "Maya", Text: "look", Attachments: []api.Attachment{att}}},
	}
	block := m.messageBlock(0)
	if len(block) != 2 {
		t.Fatalf("block = %v, want main line + loading line", block)
	}
	if !strings.Contains(block[1], "pic.jpg") || !strings.Contains(block[1], "loading") {
		t.Errorf("loading line = %q, want filename and loading marker", block[1])
	}

	m.mediaPreviews = map[string]mediaPreview{"mxc://x/1": {lines: []string{"row1", "row2"}}}
	block = m.messageBlock(0)
	if len(block) != 4 {
		t.Fatalf("block = %v, want main + 2 preview rows + caption", block)
	}
	if !strings.Contains(block[3], "2.0 MB") || !strings.Contains(block[3], "o: open") {
		t.Errorf("caption = %q, want size and open hint", block[3])
	}
}

func TestMessageBlock_VoiceNoteShowsTranscription(t *testing.T) {
	att := api.Attachment{Type: "audio", IsVoiceNote: true, Duration: 74, Transcription: "order without me"}
	m := Model{
		mode:     ModeConversation,
		messages: []api.Message{{ID: "m1", SenderName: "Maya", Attachments: []api.Attachment{att}}},
	}
	block := m.messageBlock(0)
	if len(block) != 3 {
		t.Fatalf("block = %v, want main + voice + transcription", block)
	}
	if !strings.Contains(block[1], "voice note 1:14") {
		t.Errorf("voice line = %q, want duration 1:14", block[1])
	}
	if !strings.Contains(block[2], "order without me") {
		t.Errorf("transcription line = %q", block[2])
	}
}

func TestMessageBlock_VideoShowsLabel(t *testing.T) {
	att := api.Attachment{Type: "video", Duration: 32, FileName: "beach.mov"}
	m := Model{
		mode:     ModeConversation,
		messages: []api.Message{{ID: "m1", SenderName: "Dad", Attachments: []api.Attachment{att}}},
	}
	block := m.messageBlock(0)
	if len(block) != 2 || !strings.Contains(block[1], "video 0:32") || !strings.Contains(block[1], "beach.mov") {
		t.Errorf("video block = %v, want one label line with duration and name", block)
	}
}

func TestMaxMsgOffset_CountsAttachmentLines(t *testing.T) {
	att := api.Attachment{Type: "img", ID: "mxc://x/1", FileName: "pic.jpg"}
	m := Model{
		mode:   ModeConversation,
		height: 8, // visibleRows = 6
		messages: []api.Message{
			{ID: "m1", Text: "one"},
			{ID: "m2", Text: "two", Attachments: []api.Attachment{att}},
			{ID: "m3", Text: "three"},
		},
		mediaPreviews: map[string]mediaPreview{"mxc://x/1": {lines: []string{"r1", "r2", "r3"}}},
	}
	// Heights: m1=1, m2=5 (main + 3 preview + caption), m3=1. From m1 that is
	// 7 lines > 6, so the bottom pin lands on m2.
	if got := m.maxMsgOffset(); got != 1 {
		t.Errorf("maxMsgOffset() = %d, want 1", got)
	}
}

func TestHandleInsertKey_BackspaceEatsChipWhenInputEmpty(t *testing.T) {
	m := Model{mode: ModeInsert, composeAtts: []string{"/tmp/a.png", "/tmp/b.png"}}
	m, _ = m.handleInsertKey("backspace", "")
	if len(m.composeAtts) != 1 {
		t.Fatalf("composeAtts = %v, want last chip removed", m.composeAtts)
	}
	m.input = "hi"
	m, _ = m.handleInsertKey("backspace", "")
	if m.input != "h" || len(m.composeAtts) != 1 {
		t.Errorf("backspace with text must edit text, got input=%q atts=%v", m.input, m.composeAtts)
	}
}

func TestHandleInsertKey_CtrlVFiresClipboardCmd(t *testing.T) {
	m := Model{mode: ModeInsert}
	_, cmd := m.handleInsertKey("ctrl+v", "")
	if cmd == nil {
		t.Error("ctrl+v should return a clipboard command")
	}
}

func TestHandlePaste_InsertModeChipsMediaPathAndKeepsText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := Model{mode: ModeInsert}
	m, _ = m.handlePaste(path)
	if len(m.composeAtts) != 1 || m.input != "" {
		t.Fatalf("pasting a media path should chip it: atts=%v input=%q", m.composeAtts, m.input)
	}
	m, _ = m.handlePaste("plain words")
	if m.input != "plain words" {
		t.Errorf("pasting text should append to input, got %q", m.input)
	}
}

func TestSendInput_WithAttachmentsSendsAndClearsChips(t *testing.T) {
	m := Model{
		mode:          ModeInsert,
		currentChatID: "chat-1",
		input:         "caption",
		composeAtts:   []string{"/tmp/a.png"},
		failedSends:   map[string]failedSend{},
	}
	m, cmd := m.sendInput()
	if cmd == nil {
		t.Fatal("sendInput with attachments must return a send command")
	}
	if m.mode != ModeConversation || m.input != "" || len(m.composeAtts) != 0 {
		t.Errorf("compose state not reset: mode=%v input=%q atts=%v", m.mode, m.input, m.composeAtts)
	}
	last := m.messages[len(m.messages)-1]
	if !last.IsFromMe || last.Text != "caption" || len(last.Attachments) != 1 {
		t.Errorf("optimistic message = %+v, want caption and one attachment", last)
	}
}

func TestSendInput_AttachmentOnlyNeedsNoText(t *testing.T) {
	m := Model{mode: ModeInsert, currentChatID: "chat-1", composeAtts: []string{"/tmp/a.png"}}
	m, cmd := m.sendInput()
	if cmd == nil || len(m.messages) != 1 {
		t.Errorf("attachment-only send should fire: cmd=%v msgs=%d", cmd, len(m.messages))
	}
}

func TestComposeChips_LabelsByKind(t *testing.T) {
	m := Model{composeAtts: []string{"/tmp/a.png", "/tmp/b.mov"}}
	got := m.composeChips()
	if got != "[image #1] [video #2] " {
		t.Errorf("composeChips() = %q", got)
	}
}

func TestRender_InsertShowsChips(t *testing.T) {
	m := Model{
		mode:          ModeInsert,
		currentChatID: "chat-1",
		chats:         []api.Chat{{ID: "chat-1", Title: "Test Chat"}},
		composeAtts:   []string{"/tmp/a.png"},
		input:         "hi",
		width:         80, height: 24,
	}
	if out := m.render(); !strings.Contains(out, "[image #1] hi") {
		t.Errorf("insert view missing chip before input: %q", out)
	}
}

func TestFormatBytes(t *testing.T) {
	for n, want := range map[int64]string{0: "", 512: "512 B", 4096: "4 KB", 3 << 20: "3.0 MB"} {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestOpenablePath_AddsExtensionForBareCacheFile(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, "asset123")
	if err := os.WriteFile(bare, []byte("jpegbytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := openablePath(bare, api.Attachment{FileName: "pic.jpg"})
	if filepath.Ext(got) != ".jpg" {
		t.Errorf("openablePath() = %q, want a .jpg path", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("linked path missing: %v", err)
	}
}

func TestOpenablePath_KeepsExistingExtension(t *testing.T) {
	if got := openablePath("/cache/a.png", api.Attachment{FileName: "b.jpg"}); got != "/cache/a.png" {
		t.Errorf("openablePath() = %q, want untouched path", got)
	}
}

func TestMessageBlock_SelectedMessageGetsAttachmentGutter(t *testing.T) {
	att := api.Attachment{Type: "video", Duration: 5, FileName: "c.mov"}
	m := Model{
		mode:        ModeConversation,
		msgSelected: 0,
		messages:    []api.Message{{ID: "m1", Attachments: []api.Attachment{att}}},
	}
	block := m.messageBlock(0)
	if !strings.Contains(block[1], "▌") {
		t.Errorf("selected attachment line = %q, want the ▌ gutter", block[1])
	}
	m.msgSelected = 1
	if block = m.messageBlock(0); strings.Contains(block[1], "▌") {
		t.Errorf("unselected attachment line = %q, want no gutter", block[1])
	}
}

func TestApplyMessagesRefreshed_EchoClearsFailedSend(t *testing.T) {
	now := time.Now()
	local := api.Message{
		ID: "local:1", ChatID: "chat-1", Text: "caption", IsFromMe: true,
		Timestamp:   now,
		Attachments: []api.Attachment{{Type: "unknown", FileName: "a.png"}},
	}
	m := Model{
		mode:          ModeConversation,
		currentChatID: "chat-1",
		height:        24,
		messages:      []api.Message{local},
		failedSends:   map[string]failedSend{"local:1": {}},
	}
	echo := api.Message{
		ID: "srv-1", ChatID: "chat-1", Text: "caption", IsFromMe: true,
		Timestamp:   now.Add(2 * time.Second),
		Attachments: []api.Attachment{{Type: "img", FileName: "a.png"}},
	}
	m = m.applyMessagesRefreshed(messagesRefreshedMsg{chatID: "chat-1", messages: []api.Message{echo}})
	if len(m.messages) != 1 || m.messages[0].ID != "srv-1" {
		t.Fatalf("messages = %+v, want only the server echo", m.messages)
	}
	if _, ok := m.failedSends["local:1"]; ok {
		t.Error("failed flag should clear once the server carries the message")
	}
}

func TestApplyMessagesRefreshed_KeepsGenuinelyUnsentLocal(t *testing.T) {
	now := time.Now()
	local := api.Message{
		ID: "local:1", ChatID: "chat-1", Text: "", IsFromMe: true,
		Timestamp:   now,
		Attachments: []api.Attachment{{Type: "unknown", FileName: "a.png"}},
	}
	// An old own attachment message with the same empty text must not be
	// mistaken for the echo.
	stale := api.Message{
		ID: "srv-0", ChatID: "chat-1", Text: "", IsFromMe: true,
		Timestamp:   now.Add(-1 * time.Hour),
		Attachments: []api.Attachment{{Type: "img", FileName: "old.png"}},
	}
	m := Model{
		mode:          ModeConversation,
		currentChatID: "chat-1",
		height:        24,
		messages:      []api.Message{stale, local},
		failedSends:   map[string]failedSend{"local:1": {}},
	}
	m = m.applyMessagesRefreshed(messagesRefreshedMsg{chatID: "chat-1", messages: []api.Message{stale}})
	if len(m.messages) != 2 {
		t.Fatalf("messages = %+v, want stale + re-appended local", m.messages)
	}
	if _, ok := m.failedSends["local:1"]; !ok {
		t.Error("failed flag must survive while the server lacks the message")
	}
}
