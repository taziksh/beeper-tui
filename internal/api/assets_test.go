package api_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadAsset_LocalURLsPassThrough(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s for a local URL", r.URL.Path)
	})

	for input, want := range map[string]string{
		"file:///tmp/pic.png": "/tmp/pic.png",
		"/tmp/pic.png":        "/tmp/pic.png",
	} {
		got, err := client.DownloadAsset(context.Background(), input)
		if err != nil {
			t.Fatalf("DownloadAsset(%q) error = %v", input, err)
		}
		if got != want {
			t.Errorf("DownloadAsset(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDownloadAsset_ResolvesMxcViaEndpoint(t *testing.T) {
	var gotPath, gotBody string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"srcURL":"file:///cache/asset.jpg"}`))
	})

	got, err := client.DownloadAsset(context.Background(), "mxc://example/abc")
	if err != nil {
		t.Fatalf("DownloadAsset() error = %v", err)
	}
	if gotPath != "/v1/assets/download" {
		t.Errorf("path = %q, want /v1/assets/download", gotPath)
	}
	if !strings.Contains(gotBody, "mxc://example/abc") {
		t.Errorf("body = %q, want it to contain the mxc URL", gotBody)
	}
	if got != "/cache/asset.jpg" {
		t.Errorf("DownloadAsset() = %q, want /cache/asset.jpg", got)
	}
}

func TestDownloadAsset_SurfacesServerError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})

	_, err := client.DownloadAsset(context.Background(), "mxc://example/missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("DownloadAsset() error = %v, want it to surface 'not found'", err)
	}
}

func TestUploadAsset_PostsFileAndReturnsUploadID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(path, []byte("fake-png-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotPath, gotContentType, gotBody string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uploadID":"up-1","fileName":"photo.png"}`))
	})

	id, err := client.UploadAsset(context.Background(), path)
	if err != nil {
		t.Fatalf("UploadAsset() error = %v", err)
	}
	if id != "up-1" {
		t.Errorf("uploadID = %q, want up-1", id)
	}
	if gotPath != "/v1/assets/upload" {
		t.Errorf("path = %q, want /v1/assets/upload", gotPath)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("content type = %q, want multipart/form-data", gotContentType)
	}
	if !strings.Contains(gotBody, "fake-png-bytes") || !strings.Contains(gotBody, "photo.png") {
		t.Errorf("body missing file bytes or filename")
	}
}

func TestSendAttachment_PostsUploadIDAndCaption(t *testing.T) {
	var gotBody string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chatID":"chat-1","pendingMessageID":"pending-1"}`))
	})

	err := client.SendAttachment(context.Background(), "chat-1", "look at this", "up-1")
	if err != nil {
		t.Fatalf("SendAttachment() error = %v", err)
	}
	if !strings.Contains(gotBody, `"uploadID":"up-1"`) {
		t.Errorf("body = %q, want it to contain the uploadID", gotBody)
	}
	if !strings.Contains(gotBody, "look at this") {
		t.Errorf("body = %q, want it to contain the caption", gotBody)
	}
}
