package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/identity"
	"github.com/taziksh/beeper-tui/internal/redact"
)

func testVault() *redact.Vault {
	return redact.NewVault([]identity.Person{{
		Name:     "Dana Kim",
		Username: "@dana",
		Phone:    "+15551234567",
		Email:    "dana@example.com",
	}})
}

// leakingMsgs carries every identity variant through every outbound field.
func leakingMsgs() []Message {
	return []Message{
		{Role: "system", Content: "The user's friend is Dana Kim."},
		{Role: "user", Content: "what did Dana say? her number is +15551234567"},
		{Role: "assistant", ToolCalls: []ToolCall{{
			ID: "c1", Type: "function",
			Function: FuncCall{Name: "search_messages", Arguments: `{"sender":"Dana Kim"}`},
		}}},
		{Role: "tool", Content: "Dana Kim: dinner at 7, mail me at dana@example.com", ToolCallID: "c1"},
	}
}

func assertNoIdentity(t *testing.T, body string) {
	t.Helper()
	for _, raw := range []string{"Dana", "Kim", "@dana", "+15551234567", "dana@example.com"} {
		if strings.Contains(body, raw) {
			t.Errorf("request body leaks %q", raw)
		}
	}
	if !strings.Contains(body, "CONTACT_") {
		t.Error("request body has no tokens; redaction did not run")
	}
}

func TestStreamRedactsOutboundAndRehydratesInbound(t *testing.T) {
	vault := testVault()
	token := vault.Redact("Dana Kim")
	if !strings.HasPrefix(token, "CONTACT_") {
		t.Fatalf("vault token = %q", token)
	}

	var body strings.Builder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 1<<20)
		n, _ := r.Body.Read(b)
		body.Write(b[:n])
		w.Header().Set("Content-Type", "text/event-stream")
		// Split the token mid-boundary across deltas.
		for _, part := range []string{token[:4], token[4:] + " said hi", ` {"sender":"` + token + `"}`} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", part)
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c2\",\"function\":{\"name\":\"person_card\",\"arguments\":%q}}]}}]}\n\n", `{"person":"`+token+`"}`)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := New(srv.URL+"/v1", "m", WithRedactor(vault))
	var deltas strings.Builder
	msg, err := c.Stream(context.Background(), leakingMsgs(), nil,
		StreamHandlers{OnDelta: func(s string) { deltas.WriteString(s) }})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	assertNoIdentity(t, body.String())
	want := `Dana Kim said hi {"sender":"Dana Kim"}`
	if deltas.String() != want {
		t.Errorf("deltas = %q, want %q", deltas.String(), want)
	}
	if msg.Content != want {
		t.Errorf("content = %q, want %q", msg.Content, want)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Arguments != `{"person":"Dana Kim"}` {
		t.Errorf("tool calls = %+v, want rehydrated arguments", msg.ToolCalls)
	}
}

func TestStreamDoesNotMutateCallerMessages(t *testing.T) {
	srv := sseServer(t, []string{`{"choices":[{"delta":{"content":"ok"}}]}`})
	defer srv.Close()

	msgs := leakingMsgs()
	c := New(srv.URL+"/v1", "m", WithRedactor(testVault()))
	if _, err := c.Stream(context.Background(), msgs, nil, StreamHandlers{}); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if msgs[0].Content != "The user's friend is Dana Kim." {
		t.Errorf("caller message mutated: %q", msgs[0].Content)
	}
	if msgs[2].ToolCalls[0].Function.Arguments != `{"sender":"Dana Kim"}` {
		t.Errorf("caller tool call mutated: %q", msgs[2].ToolCalls[0].Function.Arguments)
	}
}

func TestCompleteRedactsOutboundAndRehydratesInbound(t *testing.T) {
	vault := testVault()
	token := vault.Redact("Dana Kim")

	var body strings.Builder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 1<<20)
		n, _ := r.Body.Read(b)
		body.Write(b[:n])
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, `{"quote":"`+token+` was here"}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "m", WithRedactor(vault))
	out, err := c.Complete(context.Background(), leakingMsgs(), "t", map[string]any{"type": "object"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	assertNoIdentity(t, body.String())
	if want := `{"quote":"Dana Kim was here"}`; out != want {
		t.Errorf("Complete = %q, want %q", out, want)
	}
}
