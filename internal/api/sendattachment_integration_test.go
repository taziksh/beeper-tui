//go:build integration

package api_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

// TestIntegration_SendAttachment reproduces the composer's attachment flow
// against the live API: upload a synthetic 1x1 PNG, then send it with a
// caption to the note-to-self chat. Logs never include chat data.
func TestIntegration_SendAttachment(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Token == "" {
		t.Skip("BEEPER_ACCESS_TOKEN not set; skipping integration test")
	}
	client := api.New(cfg)
	ctx := context.Background()

	chats, err := client.ListChats(ctx)
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	self := ""
	re := regexp.MustCompile(`(?i)note to self`)
	for _, c := range chats {
		if re.MatchString(c.Title) {
			self = c.ID
			break
		}
	}
	if self == "" {
		t.Skip("no note-to-self chat found; skipping")
	}

	path := filepath.Join(t.TempDir(), "tiny.png")
	if err := os.WriteFile(path, tinyPNG(), 0o600); err != nil {
		t.Fatal(err)
	}

	uploadID, err := client.UploadAsset(ctx, path)
	if err != nil {
		t.Fatalf("UploadAsset() error = %v", err)
	}
	t.Logf("upload ok, id length %d", len(uploadID))

	if err := client.SendAttachment(ctx, self, "integration test caption", uploadID); err != nil {
		t.Fatalf("SendAttachment() error = %v", err)
	}
	t.Log("send with caption ok")
}

func tinyPNG() []byte {
	chunk := func(typ string, data []byte) []byte {
		var b bytes.Buffer
		binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.WriteString(typ)
		b.Write(data)
		binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(typ), data...)))
		return b.Bytes()
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], 1)
	binary.BigEndian.PutUint32(ihdr[4:], 1)
	ihdr[8], ihdr[9] = 8, 2
	var idat bytes.Buffer
	zw := zlib.NewWriter(&idat)
	zw.Write([]byte{0, 0xff, 0, 0})
	zw.Close()
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	b.Write(chunk("IHDR", ihdr))
	b.Write(chunk("IDAT", idat.Bytes()))
	b.Write(chunk("IEND", nil))
	return b.Bytes()
}
