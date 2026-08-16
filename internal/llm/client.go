// Package llm is a minimal client for OpenAI-compatible chat-completion
// servers such as LM Studio and Ollama. It supports streaming responses and
// tool calls with no dependencies beyond the standard library, so the same
// client can later point at any compatible remote endpoint.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Client talks to one OpenAI-compatible server.
type Client struct {
	httpc   *http.Client
	baseURL string // e.g. http://127.0.0.1:1234/v1
	model   string
	apiKey  string
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the default transport. Remote providers pass a
// verified pinned client here.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpc = hc }
}

// WithAPIKey sends the key as a bearer token on every request.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// New builds a client. model may be empty, in which case DetectModel picks
// the first loaded non-embedding model.
func New(baseURL, model string, opts ...Option) *Client {
	c := &Client{
		httpc:   &http.Client{},
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// newRequest builds a request with the auth and content headers every
// endpoint call needs.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return req, nil
}

// Model returns the model id requests use.
func (c *Client) Model() string { return c.model }

// BaseURL returns the OpenAI-compatible endpoint requests use.
func (c *Client) BaseURL() string { return c.baseURL }

// SetModel overrides the model id.
func (c *Client) SetModel(id string) { c.model = id }

// DetectModel asks the server for its model list. It keeps an explicitly
// configured model; otherwise it picks and stores the first non-embedding
// model. In both cases the request doubles as an endpoint availability check.
func (c *Client) DetectModel(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := c.newRequest(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("models: HTTP %d", resp.StatusCode)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	// A configured model is authoritative. We still call /models so entering
	// the Chat tab verifies that the optional local server is reachable, but
	// must not silently replace BEEPER_LLM_MODEL with the server's first model.
	if c.model != "" {
		return c.model, nil
	}
	for _, m := range body.Data {
		if strings.Contains(strings.ToLower(m.ID), "embed") {
			continue
		}
		c.model = m.ID
		return m.ID, nil
	}
	return "", fmt.Errorf("no chat model loaded")
}

// Message is one chat-completion message.
type Message struct {
	Role       string     `json:"role"` // system | user | assistant | tool
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall is the model asking for one tool invocation.
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function FuncCall `json:"function"`
}

// FuncCall names the tool and carries its JSON-encoded arguments.
type FuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool declares one callable tool to the model.
type Tool struct {
	Type     string   `json:"type"`
	Function ToolSpec `json:"function"`
}

// ToolSpec describes a tool with a JSON-schema parameter object.
type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// streamHeaderTimeout bounds dial, TLS, and response headers for Stream.
// A variable so tests can shorten it.
var streamHeaderTimeout = 30 * time.Second

// completeTimeout bounds a whole Complete call. Local extraction over many
// messages is the slow case.
const completeTimeout = 180 * time.Second

// StreamHandlers receives incremental output during Stream. Either field may
// be nil. OnDelta gets visible answer fragments; OnReasoning gets hidden
// thinking fragments from reasoning models.
type StreamHandlers struct {
	OnDelta     func(string)
	OnReasoning func(string)
}

type chatRequest struct {
	Model          string    `json:"model"`
	Messages       []Message `json:"messages"`
	Tools          []Tool    `json:"tools,omitempty"`
	Stream         bool      `json:"stream"`
	Temperature    float64   `json:"temperature"`
	ResponseFormat any       `json:"response_format,omitempty"`
}

type chunkDelta struct {
	Content          string `json:"content"`
	Reasoning        string `json:"reasoning"`
	ReasoningContent string `json:"reasoning_content"`
	ToolCalls        []struct {
		Index    int    `json:"index"`
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

type chunk struct {
	Choices []struct {
		Delta        chunkDelta `json:"delta"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Stream sends the conversation and streams the assistant's reply, invoking
// the handlers per fragment. It returns the complete assistant message,
// including any tool calls the model requested. Cancel ctx to stop.
func (c *Client) Stream(ctx context.Context, msgs []Message, tools []Tool, h StreamHandlers) (Message, error) {
	body, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    msgs,
		Tools:       tools,
		Stream:      true,
		Temperature: 0.1,
	})
	if err != nil {
		return Message{}, err
	}
	// Bound the wait for response headers without capping how long the
	// stream itself may run. Cancelling after headers arrive would kill the
	// body read, so the timer only fires before then.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var timedOut atomic.Bool
	timer := time.AfterFunc(streamHeaderTimeout, func() {
		timedOut.Store(true)
		cancel()
	})
	req, err := c.newRequest(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	if err != nil {
		timer.Stop()
		return Message{}, err
	}
	resp, err := c.httpc.Do(req)
	timer.Stop()
	if err != nil {
		if timedOut.Load() {
			return Message{}, fmt.Errorf("chat: connect timed out")
		}
		return Message{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Message{}, fmt.Errorf("chat: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	out := Message{Role: "assistant"}
	calls := map[int]*ToolCall{}
	maxIdx := -1
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ck chunk
		if err := json.Unmarshal([]byte(data), &ck); err != nil {
			continue
		}
		if ck.Error != nil {
			return Message{}, fmt.Errorf("chat: %s", ck.Error.Message)
		}
		if len(ck.Choices) == 0 {
			continue
		}
		d := ck.Choices[0].Delta
		if d.Content != "" {
			out.Content += d.Content
			if h.OnDelta != nil {
				h.OnDelta(d.Content)
			}
		}
		if r := d.Reasoning + d.ReasoningContent; r != "" && h.OnReasoning != nil {
			h.OnReasoning(r)
		}
		for _, tc := range d.ToolCalls {
			cur := calls[tc.Index]
			if cur == nil {
				cur = &ToolCall{Type: "function"}
				calls[tc.Index] = cur
				if tc.Index > maxIdx {
					maxIdx = tc.Index
				}
			}
			if tc.ID != "" {
				cur.ID = tc.ID
			}
			if tc.Function.Name != "" {
				cur.Function.Name += tc.Function.Name
			}
			cur.Function.Arguments += tc.Function.Arguments
		}
	}
	if err := sc.Err(); err != nil {
		return Message{}, err
	}
	for i := 0; i <= maxIdx; i++ {
		if tc := calls[i]; tc != nil {
			out.ToolCalls = append(out.ToolCalls, *tc)
		}
	}
	return out, nil
}

// Complete sends the conversation without streaming and constrains the reply
// to the given JSON schema, returning the raw JSON content. Use for
// structured extraction rather than conversation.
func (c *Client) Complete(ctx context.Context, msgs []Message, schemaName string, schema map[string]any) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    msgs,
		Temperature: 0,
		ResponseFormat: map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   schemaName,
				"strict": true,
				"schema": schema,
			},
		},
	})
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, completeTimeout)
	defer cancel()
	req, err := c.newRequest(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: %s: %s", resp.Status, truncateBody(data))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("llm: decode completion: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("llm: completion returned no choices")
	}
	// Reasoning models on LM Studio sometimes emit the schema-constrained
	// JSON on the reasoning channel and leave content empty.
	msg := out.Choices[0].Message
	for _, raw := range []string{msg.Content, msg.ReasoningContent} {
		if j := extractJSONObject(raw); j != "" {
			return j, nil
		}
	}
	return "", fmt.Errorf("llm: completion had no JSON object")
}

// extractJSONObject returns the outermost {...} span of s, tolerating think
// blocks or fences around it.
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func truncateBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
