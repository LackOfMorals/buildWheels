// files.go — resolution of the license and description input files.
package main

import (
	"fmt"
	"log/slog"
	"os"
)

// resolveLicense returns the license file bytes. When cfg.LicensePath is set
// it reads from disk; otherwise it tries to fetch LICENSE.txt then LICENSE
// from the main branch of cfg.Repo.
func resolveLicense(cfg *Config) ([]byte, error) {
	if cfg.LicensePath != "" {
		data, err := os.ReadFile(cfg.LicensePath)
		if err != nil {
			return nil, fmt.Errorf("read license %s: %w", cfg.LicensePath, err)
		}
		slog.Debug("using local license", "path", cfg.LicensePath)
		return data, nil
	}

	for _, name := range []string{"LICENSE.txt", "LICENSE"} {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/%s", cfg.Repo, name)
		slog.Debug("fetching license from repo", "url", url)
		data, err := httpGet(url)
		if err == nil {
			slog.Info("fetched license from repo", "repo", cfg.Repo, "file", name)
			return data, nil
		}
		slog.Debug("license candidate not found", "file", name, "error", err)
	}
	return nil, fmt.Errorf("could not fetch license from %s (tried LICENSE.txt, LICENSE)", cfg.Repo)
}

// resolveDescription reads the long-form Markdown description from disk.
// Falls back to "DESCRIPTION.md" when descPath is empty.
func resolveDescription(descPath string) ([]byte, error) {
	if descPath == "" {
		descPath = "DESCRIPTION.md"
	}
	data, err := os.ReadFile(descPath)
	if err != nil {
		return nil, fmt.Errorf("read description %s: %w", descPath, err)
	}
	slog.Debug("using description", "path", descPath)
	return data, nil
}
