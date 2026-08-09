package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/llm"
)

// chatRole distinguishes transcript turns.
type chatRole int

const (
	chatYou chatRole = iota
	chatAssistant
)

// chatTurn is one transcript entry. Assistant turns accumulate streamed text
// and the tool steps taken while producing it.
type chatTurn struct {
	role      chatRole
	text      string
	steps     []toolStep
	streaming bool
	errText   string
}

// toolStep is one tool call in an assistant turn's trace.
type toolStep struct {
	name    string
	args    string // short human summary of the arguments
	result  string // short human summary of the outcome, empty while running
	running bool
	sources []chatSource
}

// chatSource is a message a tool surfaced, shown by the trace and split styles.
type chatSource struct {
	chatID    string
	chatTitle string
	sender    string
	snippet   string
	ts        time.Time
}

// chatSession is one in-flight assistant response. events carries progress
// from the agent goroutine into Update; cancel aborts the whole response.
type chatSession struct {
	events chan chatEvent
	cancel context.CancelFunc
}

type chatEventKind int

const (
	chatEvToken chatEventKind = iota
	chatEvReasoning
	chatEvToolStart
	chatEvToolEnd
	chatEvDone
	chatEvErr
)

type chatEvent struct {
	kind chatEventKind
	text string
	step toolStep
	err  error
}

type chatEventMsg struct{ ev chatEvent }

type chatModelMsg struct {
	id  string
	err error
}

// maxChatSteps bounds the tool-call loop for one response.
const maxChatSteps = 8

// chatSystemPrompt grounds the assistant. The date matters: questions like
// "this week" are relative, and the model's own sense of today is stale.
func chatSystemPrompt() string {
	return fmt.Sprintf(`You are the assistant inside beeper-tui, the user's terminal messaging client. You answer questions about their real chats and messages using the provided tools.

Rules:
- Always look facts up with tools before answering; never invent chats, people, or messages.
- Today is %s.
- For questions about owed replies or follow-ups, call unanswered_chats first.
- To identify a person, try search_contacts before digging through messages.
- search_messages matches literal words: search "dinner" not "dinner plans", and prefer date/sender/chat filters over broad queries.
- Answer literally, at the granularity asked. "Who texted me" wants names, not message contents; expand only when asked.
- Be concise. When a fact comes from a message, say which chat and roughly when.
- person_card shows what the user remembers about someone; update_person_card re-extracts facts from that person's recent messages. Use them for questions about who a person is.
- If a tool errors or the data isn't there, say so plainly.`, time.Now().Format("Monday, January 2, 2006"))
}

// enterChatTab switches into the chat tab and kicks off model detection on
// first visit.
func (m Model) enterChatTab() (Model, tea.Cmd) {
	m.mode = ModeChat
	if m.llm == nil {
		return m, nil // no LLM configured (tests); renderChat shows setup help
	}
	if m.chatModel == "" && m.llm.Model() != "" {
		m.chatModel = m.llm.Model()
	}
	if m.chatModel == "" && !m.chatDetecting {
		m.chatDetecting = true
		return m, m.detectModelCmd()
	}
	return m, nil
}

func (m Model) detectModelCmd() tea.Cmd {
	client := m.llm
	return func() tea.Msg {
		id, err := client.DetectModel(context.Background())
		return chatModelMsg{id: id, err: err}
	}
}

// submitChatInput sends the typed question: appends the user turn plus a
// streaming assistant turn and starts the session goroutine.
func (m Model) submitChatInput() (Model, tea.Cmd) {
	if m.chatInput == "" || m.chatSession != nil || m.llm == nil {
		return m, nil
	}
	history := m.chatHistory()
	history = append(history, llm.Message{Role: "user", Content: m.chatInput})
	m.chatTurns = append(m.chatTurns,
		chatTurn{role: chatYou, text: m.chatInput},
		chatTurn{role: chatAssistant, streaming: true},
	)
	m.chatInput = ""
	m.chatTokens = 0
	m.chatReasoning = 0
	m.chatStarted = time.Now()
	m.chatFollow = true
	m.mode = ModeChat

	ctx, cancel := context.WithCancel(context.Background())
	session := &chatSession{events: make(chan chatEvent, 64), cancel: cancel}
	m.chatSession = session
	go runChatSession(ctx, m.llm, toolEnv{client: m.client, llm: m.llm, people: m.people}, history, session.events)
	return m, m.waitForChatEvent()
}

// chatHistory rebuilds the LLM conversation from finished transcript turns.
// Tool traffic is not replayed; the model re-fetches if it needs data again.
func (m Model) chatHistory() []llm.Message {
	msgs := []llm.Message{{Role: "system", Content: chatSystemPrompt()}}
	for _, t := range m.chatTurns {
		if t.errText != "" || t.streaming {
			continue
		}
		role := "assistant"
		if t.role == chatYou {
			role = "user"
		}
		if t.text == "" {
			continue
		}
		msgs = append(msgs, llm.Message{Role: role, Content: t.text})
	}
	return msgs
}

// runChatSession is the agent loop: stream a reply, execute any tool calls,
// feed results back, and repeat until the model answers in text. It owns the
// events channel and closes it when done.
func runChatSession(ctx context.Context, lc *llm.Client, env toolEnv, msgs []llm.Message, events chan<- chatEvent) {
	defer close(events)
	emit := func(ev chatEvent) {
		select {
		case events <- ev:
		case <-ctx.Done():
		}
	}
	for step := 0; step < maxChatSteps; step++ {
		reply, err := lc.Stream(ctx, msgs, chatTools, llm.StreamHandlers{
			OnDelta:     func(s string) { emit(chatEvent{kind: chatEvToken, text: s}) },
			OnReasoning: func(s string) { emit(chatEvent{kind: chatEvReasoning, text: s}) },
		})
		if err != nil {
			if ctx.Err() == nil {
				emit(chatEvent{kind: chatEvErr, err: err})
			}
			return
		}
		if len(reply.ToolCalls) == 0 {
			emit(chatEvent{kind: chatEvDone})
			return
		}
		msgs = append(msgs, reply)
		for _, call := range reply.ToolCalls {
			emit(chatEvent{kind: chatEvToolStart, step: toolStep{
				name:    call.Function.Name,
				args:    toolArgsSummary(call),
				running: true,
			}})
			result, done := execChatTool(ctx, env, call)
			if ctx.Err() != nil {
				return
			}
			emit(chatEvent{kind: chatEvToolEnd, step: done})
			msgs = append(msgs, llm.Message{Role: "tool", Content: result, ToolCallID: call.ID})
		}
	}
	emit(chatEvent{kind: chatEvErr, err: fmt.Errorf("stopped after %d tool rounds", maxChatSteps)})
}

// toolArgsSummary condenses a call's JSON arguments for the trace line.
func toolArgsSummary(call llm.ToolCall) string {
	var p struct {
		Chat   string `json:"chat"`
		Query  string `json:"query"`
		Filter string `json:"filter"`
	}
	parseToolArgs(call.Function.Arguments, &p)
	for _, s := range []string{p.Query, p.Chat, p.Filter} {
		if s != "" {
			return truncate(s, 30)
		}
	}
	return ""
}

// waitForChatEvent delivers one session event to Update and re-arms there.
func (m Model) waitForChatEvent() tea.Cmd {
	if m.chatSession == nil {
		return nil
	}
	ch := m.chatSession.events
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return chatEventMsg{ev: chatEvent{kind: chatEvDone}}
		}
		return chatEventMsg{ev: ev}
	}
}

// applyChatEvent folds one session event into the streaming assistant turn.
func (m Model) applyChatEvent(ev chatEvent) (Model, tea.Cmd) {
	if m.chatSession == nil || len(m.chatTurns) == 0 {
		return m, nil
	}
	turn := &m.chatTurns[len(m.chatTurns)-1]
	switch ev.kind {
	case chatEvToken:
		text := ev.text
		if turn.text == "" {
			// Reasoning models emit stray newlines before the first visible
			// token; a turn never starts with whitespace.
			text = strings.TrimLeft(text, " \n\t")
		}
		turn.text += text
		m.chatTokens++
	case chatEvReasoning:
		m.chatReasoning++
	case chatEvToolStart:
		turn.steps = append(turn.steps, ev.step)
	case chatEvToolEnd:
		// Complete the newest running step with the same name.
		for i := len(turn.steps) - 1; i >= 0; i-- {
			if turn.steps[i].running && turn.steps[i].name == ev.step.name {
				turn.steps[i] = ev.step
				break
			}
		}
	case chatEvErr:
		turn.streaming = false
		turn.errText = ev.err.Error()
		m.chatSession = nil
		return m.chatClampFollow(), nil
	case chatEvDone:
		turn.streaming = false
		m.chatSession = nil
		m.chatLinks = findChatLinks(turn.text, m.chats)
		m.chatLinkSel = -1
		return m.chatClampFollow(), nil
	}
	return m.chatClampFollow(), m.waitForChatEvent()
}

// cancelChatSession aborts the in-flight response, keeping partial text.
func (m Model) cancelChatSession() Model {
	if m.chatSession == nil {
		return m
	}
	m.chatSession.cancel()
	m.chatSession = nil
	if n := len(m.chatTurns); n > 0 && m.chatTurns[n-1].streaming {
		turn := &m.chatTurns[n-1]
		turn.streaming = false
		for i := range turn.steps {
			turn.steps[i].running = false
		}
		if turn.text == "" {
			turn.errText = "stopped"
		}
	}
	return m
}

// chatClampFollow pins the transcript to the bottom while following.
func (m Model) chatClampFollow() Model {
	if m.chatFollow {
		m.chatOffset = m.maxChatOffset()
	}
	return m.clampChatWindow()
}

// clampChatWindow keeps the scroll offset within the rendered transcript.
func (m Model) clampChatWindow() Model {
	if max := m.maxChatOffset(); m.chatOffset > max {
		m.chatOffset = max
	}
	if m.chatOffset < 0 {
		m.chatOffset = 0
	}
	return m
}

func (m Model) maxChatOffset() int {
	n := len(m.chatLines())
	vr := m.visibleRows()
	if m.mode == ModeChatInsert {
		vr-- // the input line takes one row
	}
	if n <= vr {
		return 0
	}
	return n - vr
}

// handleChatKey is the NORMAL-mode keymap inside the chat tab.
func (m Model) handleChatKey(key string) (Model, tea.Cmd) {
	if key != "g" {
		m.pendingG = false
	}
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		if m.chatLinkSel >= 0 {
			m.chatLinkSel = -1
			return m, nil
		}
		return m.cancelChatSession(), nil
	case "tab":
		if n := len(m.chatLinks); n > 0 {
			// First tab from no selection lands on 0 (-1+1)%n.
			m.chatLinkSel = (m.chatLinkSel + 1) % n
			return m, nil
		}
		return m.cycleTab(1)
	case "shift+tab":
		if n := len(m.chatLinks); n > 0 {
			// First shift+tab from no selection lands on the last link.
			if m.chatLinkSel < 0 {
				m.chatLinkSel = n - 1
			} else {
				m.chatLinkSel = (m.chatLinkSel + n - 1) % n
			}
			return m, nil
		}
		return m.cycleTab(-1)
	case "enter":
		if m.chatLinkSel >= 0 && m.chatLinkSel < len(m.chatLinks) {
			return m.openChatByID(m.chatLinks[m.chatLinkSel].chatID)
		}
		m.mode = ModeChatInsert
		return m, nil
	case "i":
		m.mode = ModeChatInsert
		return m, nil
	case "c":
		if m.chatSession == nil {
			m.chatTurns = nil
			m.chatOffset = 0
			m.chatFollow = true
		}
		return m, nil
	case "j", "down":
		m.chatFollow = false
		m.chatOffset++
		return m.clampChatWindow(), nil
	case "k", "up":
		m.chatFollow = false
		m.chatOffset--
		return m.clampChatWindow(), nil
	case "ctrl+d":
		m.chatFollow = false
		m.chatOffset += m.visibleRows() / 2
		return m.clampChatWindow(), nil
	case "ctrl+u":
		m.chatFollow = false
		m.chatOffset -= m.visibleRows() / 2
		return m.clampChatWindow(), nil
	case "G":
		m.chatFollow = true
		m.chatOffset = m.maxChatOffset()
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.chatFollow = false
			m.chatOffset = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "l", "right":
		return m.cycleTab(1)
	case "h", "left":
		return m.cycleTab(-1)
	}
	return m, nil
}

// handleChatInsertKey processes keys while typing a question.
func (m Model) handleChatInsertKey(key, text string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.chatInput = ""
		m.mode = ModeChat
		return m, nil
	case "enter":
		return m.submitChatInput()
	case "backspace":
		if r := []rune(m.chatInput); len(r) > 0 {
			m.chatInput = string(r[:len(r)-1])
		}
		return m, nil
	default:
		m.chatInput += text
		return m, nil
	}
}
