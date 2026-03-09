// github_test.go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// withMockGitHub starts a test HTTP server, points ghBaseURL at it for the
// duration of the test, and restores the original value on cleanup.
func withMockGitHub(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(func() { srv.Close() })
	orig := ghBaseURL
	ghBaseURL = srv.URL
	t.Cleanup(func() { ghBaseURL = orig })
}

func TestFetchRelease_Latest(t *testing.T) {
	want := ghRelease{
		TagName: "v1.2.3",
		Assets: []ghAsset{
			{Name: "tool_1.2.3_Linux_x86_64.tar.gz", BrowserDownloadURL: "https://example.com/a.tar.gz"},
		},
	}

	withMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/latest" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			http.Error(w, "bad Accept header", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(want)
	})

	got, err := fetchRelease("owner/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TagName != want.TagName {
		t.Errorf("TagName = %q, want %q", got.TagName, want.TagName)
	}
	if len(got.Assets) != 1 {
		t.Errorf("expected 1 asset, got %d", len(got.Assets))
	}
	if got.Assets[0].Name != want.Assets[0].Name {
		t.Errorf("asset name = %q, want %q", got.Assets[0].Name, want.Assets[0].Name)
	}
}

func TestFetchRelease_Tagged(t *testing.T) {
	want := ghRelease{TagName: "v2.0.0"}

	withMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/tags/v2.0.0" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(want)
	})

	got, err := fetchRelease("owner/repo", "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TagName != "v2.0.0" {
		t.Errorf("TagName = %q, want v2.0.0", got.TagName)
	}
}

func TestFetchRelease_404(t *testing.T) {
	withMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	_, err := fetchRelease("owner/repo", "v99.0.0")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestFetchRelease_InvalidJSON(t *testing.T) {
	withMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json {{{"))
	})

	_, err := fetchRelease("owner/repo", "")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFetchRelease_EmptyAssets(t *testing.T) {
	rel := ghRelease{TagName: "v1.0.0", Assets: []ghAsset{}}

	withMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(rel)
	})

	got, err := fetchRelease("owner/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Assets) != 0 {
		t.Errorf("expected 0 assets, got %d", len(got.Assets))
	}
}

func TestFetchRelease_MultipleAssets(t *testing.T) {
	rel := ghRelease{
		TagName: "v3.0.0",
		Assets: []ghAsset{
			{Name: "tool_Linux_x86_64.tar.gz", BrowserDownloadURL: "https://example.com/1"},
			{Name: "tool_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/2"},
			{Name: "tool_Windows_x86_64.zip", BrowserDownloadURL: "https://example.com/3"},
		},
	}

	withMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(rel)
	})

	got, err := fetchRelease("owner/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Assets) != 3 {
		t.Errorf("expected 3 assets, got %d", len(got.Assets))
	}
}
