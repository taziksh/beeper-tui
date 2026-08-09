package redact

import (
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/identity"
)

func testVault() *Vault {
	return NewVault([]identity.Person{
		{Name: "Dana Fixture", Username: "@dana", Phone: "+15550000002", Email: "dana@example.test"},
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

func TestRedactSamePersonSameToken(t *testing.T) {
	v := testVault()
	out := v.Redact("Dana Fixture met dana")
	parts := strings.Fields(out)
	if parts[0] != parts[2] {
		t.Errorf("same person got different tokens: %s", out)
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
