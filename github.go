// github.go — GitHub Releases API client.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

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

// githubClient is an authenticated GitHub REST API client.
type githubClient struct {
	baseURL string
	client  *http.Client
}

// newGithubClient returns a client targeting the public GitHub API.
func newGithubClient() *githubClient {
	return &githubClient{
		baseURL: "https://api.github.com",
		client:  httpClient,
	}
}

// get performs an authenticated GET to the GitHub REST API.
func (c *githubClient) get(ctx context.Context, repo, urlPath string) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, repo, urlPath)
	slog.Debug("github api request", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.client.Do(req)
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
func (c *githubClient) fetchRelease(ctx context.Context, repo, tag string) (ghRelease, error) {
	var (
		rel  ghRelease
		data []byte
		err  error
	)

	if tag == "" {
		slog.Info("fetching latest release", "repo", repo)
		data, err = c.get(ctx, repo, "releases/latest")
	} else {
		slog.Info("fetching release", "repo", repo, "tag", tag)
		data, err = c.get(ctx, repo, "releases/tags/"+tag)
	}
	if err != nil {
		return rel, err
	}
	return rel, json.Unmarshal(data, &rel)
}
