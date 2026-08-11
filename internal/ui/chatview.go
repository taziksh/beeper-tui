package ui

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// renderChat draws the chat tab: tab bar, transcript window, the input line
// in insert mode, and the status bar.
func (m Model) renderChat() string {
	var b strings.Builder
	b.WriteString(m.tabBar() + "\n")
	lines := m.highlightChatLinks(m.chatLines())
	vr := m.visibleRows()
	if m.mode == ModeChatInsert {
		vr--
	}
	shown := 0
	for i := m.chatOffset; i < len(lines) && shown < vr; i++ {
		b.WriteString(lines[i] + "\n")
		shown++
	}
	for ; shown < vr; shown++ {
		b.WriteString("\n")
	}
	if m.mode == ModeChatInsert {
		b.WriteString("> " + m.chatInput + "█\n")
	}
	b.WriteString(m.chatStatusBar())
	return b.String()
}

// chatLines renders the whole transcript. Scrolling and clamping windows
// over these lines.
func (m Model) chatLines() []string {
	if m.llm == nil {
		return []string{
			"",
			"  Assistant isn't configured",
			"",
			"  Chat needs a separate OpenAI-compatible model server.",
		}
	}
	if len(m.chatTurns) == 0 {
		return m.chatLandingLines()
	}
	var lines []string
	for i, t := range m.chatTurns {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, m.chatTurnLines(t, m.chatTextWidth())...)
	}
	return lines
}

// chatTextWidth is the wrap width for transcript text.
func (m Model) chatTextWidth() int {
	w := m.width - chatGutterWidth
	if w < 20 {
		w = 20
	}
	return w
}

const chatGutterWidth = 7 // "  you  " / "  ●    "

// chatTurnLines renders one turn at the given text width: the assistant's
// tool steps as inline one-liners, then gutter-labelled text.
func (m Model) chatTurnLines(t chatTurn, width int) []string {
	var lines []string
	gutter := "  you  "
	if t.role == chatAssistant {
		gutter = accentStyle.Render("  ●    ")
	}
	text := t.text
	if t.role == chatAssistant && t.streaming && text == "" && len(t.steps) == 0 {
		text = m.chatThinkingLabel()
	}

	// The assistant's tool steps render above its text, in call order.
	if t.role == chatAssistant {
		for _, s := range t.steps {
			lines = append(lines, chatStepLine(s))
		}
	}

	for i, l := range wrapLines(text, width) {
		if i == 0 {
			lines = append(lines, gutter+l)
		} else {
			lines = append(lines, strings.Repeat(" ", chatGutterWidth)+l)
		}
	}
	if t.streaming && t.text != "" {
		if n := len(lines); n > 0 {
			lines[n-1] += "█"
		}
	}
	if t.errText != "" {
		for i, line := range wrapLines(t.errText, width-2) {
			prefix := "       ! "
			if i > 0 {
				prefix = "         "
			}
			lines = append(lines, prefix+line)
		}
	}
	return lines
}

// chatLandingLines uses progressive disclosure: the healthy empty state is
// about what Chat can do; runtime setup details appear only while connecting
// or when the optional model server needs attention.
func (m Model) chatLandingLines() []string {
	if m.chatDetecting || !m.chatChecked {
		return m.chatConnectingLines()
	}
	if m.chatErr != nil {
		return m.chatUnavailableLines(m.chatErr)
	}
	return m.chatReadyLines()
}

func (m Model) chatReadyLines() []string {
	server := m.chatServer()
	model := m.chatModel
	if model == "" {
		model = "auto-detected model"
	}
	lines := []string{"", "  Ask about your messages", ""}
	lines = append(lines, m.chatBodyLines("Search conversations, trace follow-ups, or ask about something you remember.")...)
	lines = append(lines, "")
	lines = append(lines, m.chatBodyLines(fmt.Sprintf("● %s · %s", server.name, model))...)
	return append(lines, "", "  › Press enter to ask")
}

func (m Model) chatConnectingLines() []string {
	server := m.chatServer()
	lines := []string{"", fmt.Sprintf("  Connecting to %s…", server.name), ""}
	lines = append(lines, m.chatBodyLines("Chat uses a separate model server. The rest of beeper-tui works without it.")...)
	lines = append(lines, "")
	return append(lines, m.chatBodyLines(server.address)...)
}

func (m Model) chatUnavailableLines(err error) []string {
	server := m.chatServer()
	failure := chatFailureFor(err, server)
	lines := []string{"", "  ! " + failure.title, ""}
	lines = append(lines, m.chatBodyLines(failure.summary)...)
	lines = append(lines, m.chatBodyLines(failure.action)...)
	lines = append(lines, "", "  › Press enter to try again", "")
	if failure.command != "" {
		lines = append(lines, m.chatBodyLines("Command  "+failure.command)...)
	}
	lines = append(lines, m.chatBodyLines("Error    "+chatErrorDetail(err))...)
	lines = append(lines, m.chatBodyLines(fmt.Sprintf("Server   %s · %s", server.name, server.address))...)
	if failure.config != "" {
		lines = append(lines, m.chatBodyLines("Config   "+failure.config)...)
	}
	return lines
}

// chatBodyLines wraps secondary copy inside the two-column body inset.
func (m Model) chatBodyLines(text string) []string {
	width := m.width - 2
	if width < 20 {
		width = 20
	}
	wrapped := wrapLines(text, width)
	for i := range wrapped {
		wrapped[i] = "  " + strings.TrimRight(wrapped[i], " ")
	}
	return wrapped
}

type chatServerInfo struct {
	name    string
	address string
}

func (m Model) chatServer() chatServerInfo {
	info := chatServerInfo{name: "model server", address: "configured endpoint"}
	if m.llm == nil {
		return info
	}
	raw := m.llm.BaseURL()
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		info.address = raw
		return info
	}
	info.address = u.Host
	if path := strings.TrimRight(u.Path, "/"); path != "" && path != "/v1" {
		info.address += path
	}
	switch u.Port() {
	case "1234":
		info.name = "LM Studio"
	case "11434":
		info.name = "Ollama"
	}
	return info
}

type chatFailure struct {
	title   string
	summary string
	action  string
	command string
	config  string
}

func chatFailureFor(err error, server chatServerInfo) chatFailure {
	detail := strings.ToLower(chatErrorDetail(err))
	switch {
	case strings.Contains(detail, "no chat model"),
		strings.Contains(detail, "model not found"),
		strings.Contains(detail, "unknown model"):
		return chatFailure{
			title:   "No chat model is available",
			summary: fmt.Sprintf("%s is responding, but Chat could not find the model it needs.", server.name),
			action:  "Load a chat model, or set BEEPER_LLM_MODEL to one the server provides.",
			config:  "BEEPER_LLM_MODEL",
		}
	case strings.Contains(detail, "connection refused"),
		strings.Contains(detail, "dial tcp"),
		strings.Contains(detail, "no such host"),
		strings.Contains(detail, "timeout"),
		strings.Contains(detail, "timed out"),
		strings.Contains(detail, "deadline exceeded"):
		action := "Start the server and make sure its OpenAI-compatible API is available."
		command := ""
		config := ""
		switch server.name {
		case "LM Studio":
			action = "Open LM Studio, load a chat model, and start Local Server."
			command = "lms server start"
			config = "BEEPER_LLM_BASE_URL"
		case "Ollama":
			action = "Start Ollama and make sure a chat model is installed."
		}
		return chatFailure{
			title:   "Can't reach " + server.name,
			summary: fmt.Sprintf("Chat depends on %s, and it is not responding.", server.name),
			action:  action,
			command: command,
			config:  config,
		}
	default:
		return chatFailure{
			title:   "The model server rejected the request",
			summary: fmt.Sprintf("Chat reached %s, but it could not complete the request.", server.name),
			action:  "Check the configured model and OpenAI-compatible chat endpoint, then try again.",
			config:  "BEEPER_LLM_MODEL · BEEPER_LLM_BASE_URL",
		}
	}
}

func chatErrorDetail(err error) string {
	if err == nil {
		return "unknown error"
	}
	detail := strings.TrimSpace(err.Error())
	lower := strings.ToLower(detail)
	if strings.Contains(lower, "connection refused") {
		return "connection refused"
	}
	if strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
		return "connection timed out"
	}
	if len(detail) > 140 {
		detail = truncate(detail, 140)
	}
	return detail
}

// chatRuntimeError is concise enough to live inside a transcript turn. The
// landing state carries the endpoint and diagnostic detail after reconnect.
func (m Model) chatRuntimeError(err error) string {
	failure := chatFailureFor(err, m.chatServer())
	return failure.title + ". " + failure.action + " Press enter to reconnect."
}

// chatThinkingLabel is the placeholder while the model reasons before any
// visible output.
func (m Model) chatThinkingLabel() string {
	if m.chatReasoning > 0 {
		return fmt.Sprintf("thinking… %d", m.chatReasoning)
	}
	return "…"
}

// chatStepLine renders one tool step as a compact one-liner.
func chatStepLine(s toolStep) string {
	label := s.name
	if s.args != "" {
		label += "(" + s.args + ")"
	}
	line := "  ⏺ " + label
	if s.running {
		line += "…"
	} else if s.result != "" {
		line += "  · " + s.result
	}
	return line
}

// wrapLines wraps text to width and splits it into lines. Empty text yields
// one empty line so gutters still render.
func wrapLines(text string, width int) []string {
	if text == "" {
		return []string{""}
	}
	return strings.Split(wrap(text, width), "\n")
}

func (m Model) chatStatusBar() string {
	if m.chatErr != nil {
		return "NORMAL  assistant offline"
	}
	if m.chatDetecting {
		return "NORMAL  connecting…"
	}
	if m.chatSession != nil {
		working := "working"
		if m.chatTokens > 0 {
			working = fmt.Sprintf("%d tok", m.chatTokens)
		} else if m.chatReasoning > 0 {
			working = fmt.Sprintf("thinking %ds", int(time.Since(m.chatStarted).Seconds()))
		}
		return fmt.Sprintf("%s  ▚ %s · esc stop", m.chatModeLabel(), working)
	}
	if m.mode == ModeChatInsert {
		return "INSERT"
	}
	return "NORMAL"
}

func (m Model) chatModeLabel() string {
	if m.mode == ModeChatInsert {
		return "INSERT"
	}
	return "NORMAL"
}
