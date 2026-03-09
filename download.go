// download.go — HTTP download with optional on-disk caching.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

// httpGet fetches url and returns the body bytes. It is the single HTTP
// primitive used by both the downloader and the license fetcher so that
// tests can rely on a single interception point.
func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// cachedDownload returns the bytes for url. When cacheDir is non-empty the
// file is stored on disk keyed by its URL basename; subsequent calls with
// the same cacheDir return the stored copy without hitting the network.
// Pass cacheDir="" to disable caching entirely.
func cachedDownload(url, cacheDir string) ([]byte, error) {
	filename := path.Base(url)

	if cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return nil, fmt.Errorf("create cache dir: %w", err)
		}
		cachePath := filepath.Join(cacheDir, filename)

		if data, err := os.ReadFile(cachePath); err == nil {
			slog.Debug("cache hit", "file", filename)
			return data, nil
		}

		slog.Info("downloading", "url", url)
		data, err := httpGet(url)
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(cachePath, data, 0o644); err != nil {
			// Non-fatal: warn but still return the data.
			slog.Warn("cache write failed", "path", cachePath, "error", err)
		} else {
			slog.Debug("cached", "path", cachePath)
		}
		return data, nil
	}

	slog.Info("downloading (no cache)", "url", url)
	return httpGet(url)
}

// defaultCacheDir returns an OS-appropriate user cache directory for this
// tool. Falls back to ".cache" in the working directory if the OS cache
// location cannot be determined.
func defaultCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "buildwheels")
	}
	return ".cache"
}
