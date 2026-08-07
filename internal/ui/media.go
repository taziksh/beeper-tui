package ui

import (
	"context"
	"fmt"
	"image"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
)

// Preview geometry. Width also adapts down to the terminal at load time.
const (
	previewMaxCols = 40
	previewMaxRows = 9
)

// mediaPreview is one attachment's rendered inline preview. A nil lines slice
// with no error means the load is still in flight.
type mediaPreview struct {
	lines []string
	err   error
}

type mediaPreviewMsg struct {
	key   string
	lines []string
	err   error
}

type openResultMsg struct{ err error }

// previewKey identifies an attachment across refetches of the same message.
func previewKey(msg api.Message, att api.Attachment) string {
	if att.ID != "" {
		return att.ID
	}
	if att.SrcURL != "" {
		return att.SrcURL
	}
	return msg.ID
}

// previewable reports whether an attachment gets an inline half-block preview.
func previewable(att api.Attachment) bool {
	return att.Type == "img"
}

// assetURL is what we hand to DownloadAsset: the mxc ID when present, else
// the source URL.
func assetURL(att api.Attachment) string {
	if att.ID != "" {
		return att.ID
	}
	return att.SrcURL
}

// loadPreviewsCmd starts preview loads for every image attachment in the open
// conversation that has no cache entry yet, marking each as in flight.
func (m *Model) loadPreviewsCmd() tea.Cmd {
	if m.mediaPreviews == nil {
		m.mediaPreviews = map[string]mediaPreview{}
	}
	cols := previewMaxCols
	if m.width > 0 && m.width-16 < cols {
		cols = m.width - 16
	}
	if cols < 8 {
		cols = 8
	}
	var cmds []tea.Cmd
	for _, msg := range m.messages {
		for _, att := range msg.Attachments {
			if !previewable(att) {
				continue
			}
			key := previewKey(msg, att)
			if _, ok := m.mediaPreviews[key]; ok {
				continue
			}
			m.mediaPreviews[key] = mediaPreview{}
			cmds = append(cmds, m.loadPreviewCmd(key, assetURL(att), cols))
		}
	}
	return tea.Batch(cmds...)
}

func (m Model) loadPreviewCmd(key, src string, cols int) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		start := time.Now()
		path, err := client.DownloadAsset(ctx, src)
		logAPIResult("preview download", start, err)
		if err != nil {
			return mediaPreviewMsg{key: key, err: err}
		}
		f, err := os.Open(path)
		if err != nil {
			return mediaPreviewMsg{key: key, err: err}
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return mediaPreviewMsg{key: key, err: err}
		}
		return mediaPreviewMsg{key: key, lines: renderHalfBlocks(img, cols, previewMaxRows)}
	}
}

// openCursorAttachment downloads the first attachment of the message under
// the cursor and opens it with the OS default app.
func (m Model) openCursorAttachment() (Model, tea.Cmd) {
	m.mediaErr = nil
	msg := m.cursorMessage()
	if msg == nil || len(msg.Attachments) == 0 {
		return m, nil
	}
	att := msg.Attachments[0]
	client := m.client
	src := assetURL(att)
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		start := time.Now()
		path, err := client.DownloadAsset(ctx, src)
		logAPIResult("open download", start, err)
		if err != nil {
			return openResultMsg{err: err}
		}
		return openResultMsg{err: exec.Command("open", openablePath(path, att)).Start()}
	}
}

// openablePath gives a downloaded asset a file extension when the cache path
// has none. macOS `open` picks the app from the extension, so an
// extensionless file lands in an arbitrary default app. The file is linked
// (or copied) into the temp dir under a name derived from the attachment's
// filename or MIME type; with no usable extension the original path is
// returned unchanged.
func openablePath(path string, att api.Attachment) string {
	if filepath.Ext(path) != "" {
		return path
	}
	ext := filepath.Ext(att.FileName)
	if ext == "" {
		if exts, err := mime.ExtensionsByType(att.MimeType); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}
	if ext == "" {
		return path
	}
	linked := filepath.Join(os.TempDir(), "beeper-tui-open-"+filepath.Base(path)+ext)
	if _, err := os.Stat(linked); err == nil {
		return linked
	}
	if err := os.Link(path, linked); err == nil {
		return linked
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return path
	}
	if err := os.WriteFile(linked, data, 0o600); err != nil {
		return path
	}
	return linked
}

// clipboardImageMsg carries the temp-file path of an image pasted with
// ctrl+v, or the path already on the clipboard when a file was copied in
// Finder.
type clipboardImageMsg struct {
	path string
	err  error
}

// pasteClipboardImageCmd extracts an image from the macOS clipboard. A
// screenshot or copied image lands as PNG data; a file copied in Finder lands
// as a file reference, which resolves to its original path.
func pasteClipboardImageCmd() tea.Cmd {
	return func() tea.Msg {
		path := filepath.Join(os.TempDir(), fmt.Sprintf("beeper-tui-paste-%d.png", time.Now().UnixNano()))
		script := fmt.Sprintf(`set f to open for access POSIX file %q with write permission
set eof of f to 0
write (the clipboard as «class PNGf») to f
close access f`, path)
		if err := exec.Command("osascript", "-e", script).Run(); err == nil {
			return clipboardImageMsg{path: path}
		}
		os.Remove(path)
		out, err := exec.Command("osascript", "-e",
			`POSIX path of (the clipboard as «class furl»)`).Output()
		if err != nil {
			return clipboardImageMsg{err: fmt.Errorf("no image on clipboard")}
		}
		p := strings.TrimSpace(string(out))
		if !isMediaFile(p) {
			return clipboardImageMsg{err: fmt.Errorf("clipboard file is not an image or video")}
		}
		return clipboardImageMsg{path: p}
	}
}

// mediaExts are the file extensions the composer accepts as attachments.
var mediaExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".heic": true, ".tiff": true, ".bmp": true,
	".mp4": true, ".mov": true, ".webm": true, ".m4a": true, ".mp3": true,
	".ogg": true, ".wav": true, ".pdf": true,
}

func isMediaFile(path string) bool {
	if !mediaExts[strings.ToLower(filepath.Ext(path))] {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// pastedFilePath recognizes a paste that is a single dragged-in file path,
// undoing the shell escaping terminals apply on drag and drop. Returns ""
// when the paste is ordinary text.
func pastedFilePath(content string) string {
	p := strings.TrimSpace(content)
	if strings.HasPrefix(p, "'") && strings.HasSuffix(p, "'") && len(p) > 1 {
		p = p[1 : len(p)-1]
	}
	p = strings.ReplaceAll(p, `\ `, " ")
	if !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "~/") {
		return ""
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		p = filepath.Join(home, p[2:])
	}
	if !isMediaFile(p) {
		return ""
	}
	return p
}

// sendAttachmentsCmd uploads each pending file and sends it, attaching the
// caption to the first. The first failure aborts the rest.
func (m Model) sendAttachmentsCmd(chatID, localID, caption string, paths []string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		for i, path := range paths {
			text := ""
			if i == 0 {
				text = caption
			}
			start := time.Now()
			uploadID, err := client.UploadAsset(ctx, path)
			logAPIResult("asset upload", start, err)
			if err != nil {
				return sendResultMsg{localID: localID, text: text, atts: paths[i:], err: err}
			}
			start = time.Now()
			err = client.SendAttachment(ctx, chatID, text, uploadID)
			logAPIResult("attachment send", start, err)
			if err != nil {
				return sendResultMsg{localID: localID, text: text, atts: paths[i:], err: err}
			}
		}
		return sendResultMsg{localID: localID}
	}
}

// formatBytes renders a size as a compact human-readable string.
func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n/(1<<10))
	case n > 0:
		return fmt.Sprintf("%d B", n)
	default:
		return ""
	}
}

// formatDuration renders seconds as m:ss.
func formatDuration(secs float64) string {
	total := int(secs + 0.5)
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}
