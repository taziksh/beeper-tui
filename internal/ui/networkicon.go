package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// brandIcon pairs a Nerd Font brand glyph with a network's brand color.
type brandIcon struct {
	glyph rune
	color string // hex brand color, or empty for the default foreground.
}

// genericChatGlyph is nf-fa-comment, used for networks without a brand glyph so
// the icon column stays one rune wide.
const genericChatGlyph = 0xf075

// networkIcons maps a lowercased network label to its brand glyph and color.
// Each glyph is a Nerd Font codepoint, so it renders as a logo in a terminal
// using a Nerd Font. The trailing nf-* names are Nerd Font's glyph identifiers.
var networkIcons = map[string]brandIcon{
	"whatsapp":   {0xf232, "#25D366"}, // nf-fa-whatsapp
	"telegram":   {0xf2c6, "#229ED9"}, // nf-fa-telegram
	"signal":     {genericChatGlyph, "#3A76F0"},
	"discord":    {0xf392, "#5865F2"}, // nf-fa-discord
	"slack":      {0xf198, "#611F69"}, // nf-fa-slack
	"instagram":  {0xf16d, "#E4405F"}, // nf-fa-instagram
	"messenger":  {0xf39f, "#0084FF"}, // nf-fa-facebook-messenger
	"facebook":   {0xf39f, "#0084FF"},
	"imessage":   {0xf179, ""}, // nf-fa-apple
	"sms":        {genericChatGlyph, "#34B7F1"},
	"linkedin":   {0xf08c, "#0A66C2"}, // nf-fa-linkedin
	"googlechat": {0xf1a0, "#00AC47"}, // nf-fa-google
	"twitter":    {0xf099, "#1DA1F2"}, // nf-fa-twitter
	"x":          {0xf099, ""},
}

// networkGlyph returns the brand glyph for network, styled in its brand color.
// The base style is merged in so the icon inherits attributes like Bold from the
// row's selection style, preventing ANSI boundary artifacts.
// Unknown networks get a neutral chat bubble so the icon column stays aligned.
func networkGlyph(network string, base lipgloss.Style) string {
	ic, ok := networkIcons[strings.ToLower(strings.TrimSpace(network))]
	if !ok {
		ic = brandIcon{glyph: genericChatGlyph}
	}
	glyph := string(ic.glyph)
	if ic.color == "" {
		return base.Render(glyph)
	}
	return base.Foreground(lipgloss.Color(ic.color)).Render(glyph)
}
