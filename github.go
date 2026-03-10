// github.go — GitHub Releases API client.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

// ghBaseURL is the GitHub API root. Overridden in tests to point at an
// httptest server.
var ghBaseURL = "https://api.github.com"

// ghAsset is one file attached to a GitHub release.
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ghRelease is the subset of GitHub release metadata we care about.
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// ghGet performs an authenticated GET to the GitHub REST API.
func ghGet(repo, urlPath string) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", ghBaseURL, repo, urlPath)
	slog.Debug("github api request", "url", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// fetchRelease returns release metadata for the given tag, or the latest
// published release when tag is empty.
func fetchRelease(repo, tag string) (ghRelease, error) {
	var (
		rel  ghRelease
		data []byte
		err  error
	)

	if tag == "" {
		slog.Info("fetching latest release", "repo", repo)
		data, err = ghGet(repo, "releases/latest")
	} else {
		slog.Info("fetching release", "repo", repo, "tag", tag)
		data, err = ghGet(repo, "releases/tags/"+tag)
	}
	if err != nil {
		return rel, err
	}
	return rel, json.Unmarshal(data, &rel)
}
