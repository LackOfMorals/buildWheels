// download_test.go
package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCachedDownload_NoCache(t *testing.T) {
	want := []byte("file content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(want)
	}))
	defer srv.Close()

	got, err := cachedDownload(srv.URL+"/file.bin", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCachedDownload_CacheMiss_Downloads(t *testing.T) {
	want := []byte("binary data")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write(want)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	got, err := cachedDownload(srv.URL+"/tool.tar.gz", cacheDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if calls != 1 {
		t.Errorf("expected 1 HTTP call, got %d", calls)
	}
}

func TestCachedDownload_CacheMiss_WritesFile(t *testing.T) {
	want := []byte("binary data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(want)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	_, err := cachedDownload(srv.URL+"/tool.tar.gz", cacheDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cached := filepath.Join(cacheDir, "tool.tar.gz")
	data, err := os.ReadFile(cached)
	if err != nil {
		t.Fatalf("expected cache file to exist: %v", err)
	}
	if string(data) != string(want) {
		t.Errorf("cached content = %q, want %q", data, want)
	}
}

func TestCachedDownload_CacheHit_NoHTTP(t *testing.T) {
	want := []byte("cached content")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte("server content — should NOT be fetched"))
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	cachePath := filepath.Join(cacheDir, "tool.tar.gz")
	if err := os.WriteFile(cachePath, want, 0o644); err != nil {
		t.Fatalf("write pre-seeded cache: %v", err)
	}

	got, err := cachedDownload(srv.URL+"/tool.tar.gz", cacheDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if calls != 0 {
		t.Errorf("cache hit should make 0 HTTP calls, got %d", calls)
	}
}

func TestCachedDownload_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := cachedDownload(srv.URL+"/file.bin", "")
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestCachedDownload_CreatesSubdirectory(t *testing.T) {
	want := []byte("data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(want)
	}))
	defer srv.Close()

	// Use a nested cache dir that doesn't exist yet.
	cacheDir := filepath.Join(t.TempDir(), "nested", "cache")
	_, err := cachedDownload(srv.URL+"/tool.tar.gz", cacheDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(cacheDir); err != nil {
		t.Errorf("cache directory should have been created: %v", err)
	}
}
