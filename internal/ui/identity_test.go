package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/identity"
)

func testIdentStore(t *testing.T) *identity.Store {
	t.Helper()
	s, err := identity.Load(filepath.Join(t.TempDir(), "identities.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestOpenIdentityCard_NewFromConversation(t *testing.T) {
	s := testIdentStore(t)
	m := Model{
		mode:          ModeConversation,
		currentChatID: "chat-1",
		chats: []api.Chat{{
			ID: "chat-1", AccountID: "acc", Network: "Signal",
			Title: "Alice", Type: "single", PeerUserID: "user-a",
		}},
		idents: s,
	}
	m = m.openIdentityCard()
	if m.mode != ModeIdentity {
		t.Fatalf("mode = %v, want ModeIdentity", m.mode)
	}
	if m.idName != "Alice" || m.idID != "" {
		t.Fatalf("draft name=%q id=%q, want Alice / empty", m.idName, m.idID)
	}
	if m.idChatID != "chat-1" || m.idPeerUserID != "user-a" {
		t.Fatalf("anchors chat=%q peer=%q", m.idChatID, m.idPeerUserID)
	}
}

func TestSaveIdentityCard_CreatesAndResolves(t *testing.T) {
	s := testIdentStore(t)
	m := Model{
		mode:          ModeConversation,
		currentChatID: "chat-1",
		chats: []api.Chat{{
			ID: "chat-1", AccountID: "acc", Network: "Signal",
			Title: "Alice", Type: "single", PeerUserID: "user-a",
		}},
		idents: s,
	}
	m = m.openIdentityCard()
	m.idName = "Alice Example"
	m.idNotes = "met at a conference"
	m, _ = m.saveIdentityCard()
	if m.mode != ModeConversation {
		t.Fatalf("after save mode = %v, want ModeConversation", m.mode)
	}

	got, ok := s.ResolveForChat("chat-1", "acc", "user-a")
	if !ok {
		t.Fatal("expected resolve after save")
	}
	if got.DisplayName != "Alice Example" || got.Notes != "met at a conference" {
		t.Fatalf("saved identity = %+v", got)
	}

	// Re-open should load existing.
	m.mode = ModeConversation
	m = m.openIdentityCard()
	if m.idID != got.ID || m.idName != "Alice Example" {
		t.Fatalf("re-open id=%q name=%q", m.idID, m.idName)
	}
}

func TestSaveIdentityCard_EmptyNameErrors(t *testing.T) {
	s := testIdentStore(t)
	m := Model{
		mode: ModeIdentity, idents: s,
		idName: "  ", idChatID: "c",
		idReturnMode: ModeList,
	}
	m, _ = m.saveIdentityCard()
	if m.mode != ModeIdentity {
		t.Fatalf("mode = %v, want stay ModeIdentity", m.mode)
	}
	if m.idErr != identity.ErrEmptyName {
		t.Fatalf("idErr = %v, want ErrEmptyName", m.idErr)
	}
}

func TestHandleIdentityKey_TabAndType(t *testing.T) {
	s := testIdentStore(t)
	m := Model{
		mode: ModeIdentity, idents: s,
		idName: "A", idFocus: idFocusName,
		idReturnMode: ModeList,
	}
	m, _ = m.handleIdentityKey("", "lice")
	if m.idName != "Alice" {
		t.Fatalf("name = %q, want Alice", m.idName)
	}
	m, _ = m.handleIdentityKey("tab", "")
	if m.idFocus != idFocusNotes {
		t.Fatalf("focus = %d, want notes", m.idFocus)
	}
	m, _ = m.handleIdentityKey("", "hi")
	m, _ = m.handleIdentityKey("enter", "")
	m, _ = m.handleIdentityKey("", "there")
	if m.idNotes != "hi\nthere" {
		t.Fatalf("notes = %q", m.idNotes)
	}
}

func TestRenderIdentity_ShowsDraft(t *testing.T) {
	m := Model{
		mode: ModeIdentity,
		idName: "Bob", idNotes: "friend",
		idChatTitle: "Bob", idNetwork: "WhatsApp",
		idFocus: idFocusName,
	}
	out := m.renderIdentity()
	if !strings.Contains(out, "IDENTITY") || !strings.Contains(out, "Bob") {
		t.Fatalf("render missing header: %q", out)
	}
	if !strings.Contains(out, "friend") {
		t.Fatalf("render missing notes: %q", out)
	}
}

func TestDeleteIdentityCard(t *testing.T) {
	s := testIdentStore(t)
	idn, err := s.Create("Alice", "x")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.LinkChat(idn.ID, "chat-1"); err != nil {
		t.Fatal(err)
	}
	m := Model{
		mode: ModeIdentity, idents: s, idID: idn.ID,
		idReturnMode: ModeList,
	}
	m, _ = m.deleteIdentityCard()
	if m.mode != ModeList {
		t.Fatalf("mode = %v", m.mode)
	}
	if _, ok := s.Get(idn.ID); ok {
		t.Fatal("identity still present after delete")
	}
}

func TestOpenIdentityCard_FromList(t *testing.T) {
	s := testIdentStore(t)
	m := Model{
		mode: ModeList, selected: 0,
		chats:  []api.Chat{{ID: "c2", Title: "Carol", Network: "iMessage"}},
		idents: s,
	}
	m = m.openIdentityCard()
	if m.mode != ModeIdentity || m.idReturnMode != ModeList {
		t.Fatalf("mode=%v return=%v", m.mode, m.idReturnMode)
	}
	if m.idName != "Carol" {
		t.Fatalf("name = %q", m.idName)
	}
}
