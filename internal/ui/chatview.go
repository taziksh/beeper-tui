package ui

import (
	"fmt"
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
			"  No local model endpoint configured.",
			"  Start LM Studio's server (lms server start) or set BEEPER_LLM_BASE_URL.",
		}
	}
	if len(m.chatTurns) == 0 {
		return nil
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
		lines = append(lines, "       ! "+truncate(t.errText, 70))
	}
	return lines
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
		return fmt.Sprintf("NORMAL  chat: %v · lms server start", m.chatErr)
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
		return "INSERT  enter ask · esc cancel"
	}
	return "NORMAL  i ask · h/l tab · q quit"
}

func (m Model) chatModeLabel() string {
	if m.mode == ModeChatInsert {
		return "INSERT"
	}
	return "NORMAL"
}
