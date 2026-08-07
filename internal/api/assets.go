package api

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// DownloadAsset resolves an attachment URL to a local file path. mxc:// and
// localmxc:// URLs are downloaded by Beeper Desktop; file:// URLs and plain
// paths are already local and pass through.
func (c *Client) DownloadAsset(ctx context.Context, srcURL string) (string, error) {
	if p, ok := localPath(srcURL); ok {
		return p, nil
	}
	res, err := c.sdk.Assets.Download(ctx, beeperdesktopapi.AssetDownloadParams{URL: srcURL})
	if err != nil {
		return "", fmt.Errorf("api: download asset: %w", err)
	}
	if res.Error != "" {
		return "", fmt.Errorf("api: download asset: %s", res.Error)
	}
	p, ok := localPath(res.SrcURL)
	if !ok {
		return "", fmt.Errorf("api: download asset: unexpected URL %q", res.SrcURL)
	}
	return p, nil
}

// localPath converts a file:// URL or a bare filesystem path to a path,
// reporting false for remote URLs.
func localPath(s string) (string, bool) {
	if strings.HasPrefix(s, "file://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", false
		}
		return u.Path, true
	}
	if strings.HasPrefix(s, "/") {
		return s, true
	}
	return "", false
}

// UploadAsset uploads a local file to Beeper Desktop's temporary storage and
// returns the upload ID to reference when sending a message.
func (c *Client) UploadAsset(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("api: upload asset: %w", err)
	}
	defer f.Close()
	res, err := c.sdk.Assets.Upload(ctx, beeperdesktopapi.AssetUploadParams{
		File:     f,
		FileName: beeperdesktopapi.String(filepath.Base(path)),
	})
	if err != nil {
		return "", fmt.Errorf("api: upload asset %s: %w", filepath.Base(path), err)
	}
	if res.Error != "" {
		return "", fmt.Errorf("api: upload asset %s: %s", filepath.Base(path), res.Error)
	}
	return res.UploadID, nil
}
