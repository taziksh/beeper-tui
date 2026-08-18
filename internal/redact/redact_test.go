package redact

import (
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/identity"
)

func testVault() *Vault {
	return NewVault([]identity.Person{
		{Name: "Dana Fixture", Usernames: []string{"@dana"}, Phones: []string{"+15550000002"}, Emails: []string{"dana@example.test"}},
		{Name: "Bob Ramírez"},
	})
}

func TestRedactCoversVariants(t *testing.T) {
	v := testVault()
	in := "Dana Fixture said dana's plan is off; text @dana or +15550000002. bob agreed."
	out := v.Redact(in)
	for _, leak := range []string{"Dana", "dana", "@dana", "+15550000002", "bob"} {
		if strings.Contains(out, leak) {
			t.Errorf("redacted text leaks %q:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, "CONTACT_") {
		t.Errorf("no tokens in output:\n%s", out)
	}
}

func TestRedactKeepsPossessive(t *testing.T) {
	v := testVault()
	out := v.Redact("dana's plan")
	if !strings.HasSuffix(strings.Fields(out)[0], "'s") {
		t.Errorf("possessive lost: %q", out)
	}
}

func TestRedactSamePersonSameToken(t *testing.T) {
	v := testVault()
	out := v.Redact("Dana Fixture met dana")
	parts := strings.Fields(out)
	if parts[0] != parts[2] {
		t.Errorf("same person got different tokens: %s", out)
	}
}

func TestRehydrateFirstNameBecomesFullDisplayName(t *testing.T) {
	v := NewVault([]identity.Person{{Name: "Lisa Wang"}})
	token := v.Redact("lisa")
	if !strings.HasPrefix(token, "CONTACT_") {
		t.Fatalf("redact(lisa) = %q", token)
	}
	if got := v.Rehydrate(token); got != "Lisa Wang" {
		t.Errorf("rehydrate(%q) = %q, want Lisa Wang", token, got)
	}
}

func TestRehydrateRoundTrip(t *testing.T) {
	v := testVault()
	token := strings.Fields(v.Redact("Dana Fixture"))[0]
	for _, form := range []string{token, strings.ToLower(token), strings.Replace(token, "_", " ", 1), strings.Replace(token, "_", "-", 1)} {
		got := v.Rehydrate("ask " + form + " about it")
		if got != "ask Dana Fixture about it" {
			t.Errorf("rehydrate %q = %q", form, got)
		}
	}
}

func TestRehydrateIgnoresInventedTokens(t *testing.T) {
	v := testVault()
	in := "CONTACT_99999999 said hi"
	if got := v.Rehydrate(in); got != in {
		t.Errorf("invented token resolved: %q", got)
	}
}

func TestRedactLeavesPlainTextAlone(t *testing.T) {
	v := testVault()
	in := "the danaher meeting is about contact lenses"
	if got := v.Redact(in); got != in {
		t.Errorf("mangled unrelated text: %q", got)
	}
}

func TestHoldBack(t *testing.T) {
	emit, hold := HoldBack("she said CONT")
	if emit != "she said " || hold != "CONT" {
		t.Errorf("HoldBack = %q + %q", emit, hold)
	}
	emit, hold = HoldBack("nothing risky here.")
	if hold != "" || emit == "" {
		t.Errorf("held back plain text: %q + %q", emit, hold)
	}
	emit, hold = HoldBack("ping CONTACT_12")
	if hold != "CONTACT_12" {
		t.Errorf("token tail not held: %q + %q", emit, hold)
	}
}

func TestUpdateLearnsNewPeopleAndKeepsTokens(t *testing.T) {
	v := testVault()
	before := v.Redact("Dana Fixture")

	v.Update([]identity.Person{
		{Name: "Dana Fixture", Phones: []string{"+15550000777"}},
		{Name: "New Person"},
	})

	if got := v.Redact("Dana Fixture"); got != before {
		t.Errorf("existing token changed: %q -> %q", before, got)
	}
	if got := v.Redact("+15550000777"); got != before {
		t.Errorf("new phone = %q, want Dana's token %q", got, before)
	}
	newTok := v.Redact("New Person")
	if !strings.HasPrefix(newTok, "CONTACT_") || newTok == before {
		t.Errorf("new person token = %q", newTok)
	}
	if got := v.Rehydrate(newTok); got != "New Person" {
		t.Errorf("Rehydrate(%q) = %q, want New Person", newTok, got)
	}
}

func TestUpdateRenameKeepsToken(t *testing.T) {
	v := testVault()
	tok := v.Redact("+15550000002")

	// The user saved the contact under a real name; the phone links them.
	v.Update([]identity.Person{{Name: "Dana Renamed", Phones: []string{"+15550000002"}}})

	if got := v.Redact("Dana Renamed"); got != tok {
		t.Errorf("renamed person = %q, want same token %q", got, tok)
	}
}
