package identity_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/taziksh/beeper-tui/internal/identity"
)

func TestLoad_MissingFileIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identities.json")
	s, err := identity.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if n := len(s.List()); n != 0 {
		t.Fatalf("len(List()) = %d, want 0", n)
	}
}

func TestCreateUpdateDelete_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identities.json")
	s, err := identity.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	idn, err := s.Create("Alice", "met at a conference")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if idn.ID == "" || idn.DisplayName != "Alice" || idn.Notes != "met at a conference" {
		t.Fatalf("Create() = %+v", idn)
	}

	// File mode must be private.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 0600", info.Mode().Perm())
	}

	// Survive reload.
	s2, err := identity.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Get(idn.ID)
	if !ok || got.DisplayName != "Alice" || got.Notes != "met at a conference" {
		t.Fatalf("after reload Get() = %+v, ok=%v", got, ok)
	}

	updated, err := s2.Update(idn.ID, "Alice Example", "likes hiking")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.DisplayName != "Alice Example" || updated.Notes != "likes hiking" {
		t.Fatalf("Update() = %+v", updated)
	}

	if err := s2.Delete(idn.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := s2.Get(idn.ID); ok {
		t.Fatal("Get after Delete: still found")
	}
	s3, err := identity.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s3.List()) != 0 {
		t.Fatalf("after delete+reload len = %d", len(s3.List()))
	}
}

func TestCreate_EmptyNameRejected(t *testing.T) {
	s, err := identity.Load(filepath.Join(t.TempDir(), "identities.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("  ", "notes"); err != identity.ErrEmptyName {
		t.Fatalf("Create empty name error = %v, want ErrEmptyName", err)
	}
}

func TestLinkAndResolve(t *testing.T) {
	s, err := identity.Load(filepath.Join(t.TempDir(), "identities.json"))
	if err != nil {
		t.Fatal(err)
	}
	alice, err := s.Create("Alice", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.LinkChat(alice.ID, "chat-1"); err != nil {
		t.Fatalf("LinkChat: %v", err)
	}
	if err := s.LinkUser(alice.ID, "acc-sig", "user-alice"); err != nil {
		t.Fatalf("LinkUser: %v", err)
	}

	if got, ok := s.ResolveByChat("chat-1"); !ok || got.ID != alice.ID {
		t.Fatalf("ResolveByChat = %+v ok=%v", got, ok)
	}
	if got, ok := s.ResolveByUser("acc-sig", "user-alice"); !ok || got.ID != alice.ID {
		t.Fatalf("ResolveByUser = %+v ok=%v", got, ok)
	}
	if got, ok := s.ResolveForChat("chat-1", "acc-sig", "user-alice"); !ok || got.ID != alice.ID {
		t.Fatalf("ResolveForChat chat = %+v ok=%v", got, ok)
	}
	if got, ok := s.ResolveForChat("other-chat", "acc-sig", "user-alice"); !ok || got.ID != alice.ID {
		t.Fatalf("ResolveForChat user fallback = %+v ok=%v", got, ok)
	}

	// Idempotent re-link.
	if err := s.LinkChat(alice.ID, "chat-1"); err != nil {
		t.Fatalf("re-LinkChat: %v", err)
	}

	bob, err := s.Create("Bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.LinkChat(bob.ID, "chat-1"); err != identity.ErrLinkTaken {
		t.Fatalf("second owner LinkChat error = %v, want ErrLinkTaken", err)
	}
	if err := s.LinkUser(bob.ID, "acc-sig", "user-alice"); err != identity.ErrLinkTaken {
		t.Fatalf("second owner LinkUser error = %v, want ErrLinkTaken", err)
	}

	if err := s.UnlinkChat(alice.ID, "chat-1"); err != nil {
		t.Fatalf("UnlinkChat: %v", err)
	}
	if _, ok := s.ResolveByChat("chat-1"); ok {
		t.Fatal("ResolveByChat after unlink: still found")
	}
	// Bob can claim the chat now.
	if err := s.LinkChat(bob.ID, "chat-1"); err != nil {
		t.Fatalf("LinkChat after unlink: %v", err)
	}
}

func TestResolve_SurvivesReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identities.json")
	s, err := identity.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	idn, err := s.Create("Carol", "works on bridge")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.LinkChat(idn.ID, "chat-carol"); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkUser(idn.ID, "acc", "u-carol"); err != nil {
		t.Fatal(err)
	}

	s2, err := identity.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.ResolveForChat("chat-carol", "acc", "u-carol")
	if !ok || got.DisplayName != "Carol" || got.Notes != "works on bridge" {
		t.Fatalf("after reload ResolveForChat = %+v ok=%v", got, ok)
	}
}
