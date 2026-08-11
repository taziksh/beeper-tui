package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sseServer(t *testing.T, lines []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, l := range lines {
			fmt.Fprintf(w, "data: %s\n\n", l)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func TestStreamContent(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"content":"Hel"}}]}`,
		`{"choices":[{"delta":{"content":"lo"}}]}`,
		`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
	})
	defer srv.Close()

	c := New(srv.URL+"/v1", "test-model")
	var got strings.Builder
	msg, err := c.Stream(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil,
		StreamHandlers{OnDelta: func(s string) { got.WriteString(s) }})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if msg.Content != "Hello" || got.String() != "Hello" {
		t.Errorf("content = %q, deltas = %q, want Hello", msg.Content, got.String())
	}
}

func TestStreamToolCalls(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"search_messages","arguments":"{\"qu"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ery\":\"cabin\"}"}}]}}]}`,
		`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
	})
	defer srv.Close()

	c := New(srv.URL+"/v1", "test-model")
	msg, err := c.Stream(context.Background(), nil, nil, StreamHandlers{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(msg.ToolCalls))
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "c1" || tc.Function.Name != "search_messages" || tc.Function.Arguments != `{"query":"cabin"}` {
		t.Errorf("tool call = %+v", tc)
	}
}

func TestStreamReasoning(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"reasoning_content":"thinking"}}]}`,
		`{"choices":[{"delta":{"content":"done"}}]}`,
	})
	defer srv.Close()

	c := New(srv.URL+"/v1", "test-model")
	var reasoning strings.Builder
	msg, err := c.Stream(context.Background(), nil, nil,
		StreamHandlers{OnReasoning: func(s string) { reasoning.WriteString(s) }})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if reasoning.String() != "thinking" {
		t.Errorf("reasoning = %q", reasoning.String())
	}
	if msg.Content != "done" {
		t.Errorf("content = %q", msg.Content)
	}
}

func TestStreamHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"model not loaded"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL+"/v1", "test-model")
	if _, err := c.Stream(context.Background(), nil, nil, StreamHandlers{}); err == nil {
		t.Fatal("want error on HTTP 404")
	}
}

func TestDetectModelSkipsEmbeddings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"text-embedding-nomic"},{"id":"qwen-test-35b"}]}`)
	}))
	defer srv.Close()

	c := New(srv.URL+"/v1", "")
	id, err := c.DetectModel(context.Background())
	if err != nil {
		t.Fatalf("DetectModel: %v", err)
	}
	if id != "qwen-test-35b" || c.Model() != "qwen-test-35b" {
		t.Errorf("model = %q", id)
	}
}

func TestDetectModelKeepsConfiguredModelWhileCheckingServer(t *testing.T) {
	requested := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		fmt.Fprint(w, `{"data":[{"id":"different-server-default"}]}`)
	}))
	defer srv.Close()

	c := New(srv.URL+"/v1", "configured-model")
	id, err := c.DetectModel(context.Background())
	if err != nil {
		t.Fatalf("DetectModel: %v", err)
	}
	if !requested {
		t.Fatal("DetectModel did not check the configured endpoint")
	}
	if id != "configured-model" || c.Model() != "configured-model" {
		t.Errorf("model = %q, want configured-model", id)
	}
}

func TestCompleteReadsReasoningChannel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","reasoning_content":"{\"city\":\"toronto\"}"}}]}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "m")
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}}, "t",
		map[string]any{"type": "object"})
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"city":"toronto"}` {
		t.Errorf("Complete = %q, want reasoning-channel JSON", out)
	}
}

func TestCompleteRejectsNoJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"sorry, no"}}]}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "m")
	if _, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}}, "t",
		map[string]any{"type": "object"}); err == nil {
		t.Fatal("Complete with no JSON object = nil error, want failure")
	}
}
