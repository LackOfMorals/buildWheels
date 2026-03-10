// main.go — CLI entry point for buildwheels.
//
// Usage:
//
//	go run . [flags]
//
// Required flags:
//
//	-repo   owner/name   GitHub repository to fetch releases from
//
// Optional flags:
//
//	-version        release tag (default: latest)
//	-binary-name    binary filename in archives (default: repo name)
//	-package-name   Python package name (default: binary-name)
//	-entry-point    console_scripts entry (default: binary-name)
//	-summary        one-line PyPI summary
//	-license-expr   SPDX license expression (default: MIT)
//	-py-version     Python package version (default: mirrors -version)
//	-output         output directory (default: ./dist)
//	-platforms      comma-separated GoReleaser OS_Arch keys (default: all)
//	-assets         comma-separated asset filenames to download (overrides auto-detect)
//	-upload         upload wheels to PyPI (default: false)
//	-pypi-url       PyPI upload endpoint (default: https://upload.pypi.org/legacy/)
//	-pypi-user      PyPI username (default: __token__)
//	-license        path to license file (default: fetch from repo)
//	-description    path to Markdown description file (default: DESCRIPTION.md)
//	-cache          binary cache directory ("" to disable; default: OS cache dir)
//	-debug          enable debug-level logging
//
// Environment variables:
//
//	PYPI_TOKEN    PyPI API token (required when -upload is set)
//	PYPI_PASSWORD alternative to PYPI_TOKEN
//	GITHUB_TOKEN  GitHub PAT to avoid API rate limits
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	cfg := &Config{}

	// GitHub source
	flag.StringVar(&cfg.Repo, "repo", "", "GitHub repository in owner/name format (required)")
	flag.StringVar(&cfg.Version, "version", "", "Release tag, e.g. v1.4.2 (default: latest)")

	// Package identity
	flag.StringVar(&cfg.BinaryName, "binary-name", "", "Binary filename inside archives (default: repo name)")
	flag.StringVar(&cfg.PackageName, "package-name", "", "Python package name (default: binary-name)")
	flag.StringVar(&cfg.EntryPoint, "entry-point", "", "console_scripts entry point (default: binary-name)")
	flag.StringVar(&cfg.Summary, "summary", "", "One-line PyPI summary (default: derived from package name)")
	flag.StringVar(&cfg.LicenseExpr, "license-expr", "MIT", "SPDX license expression")

	// Build
	flag.StringVar(&cfg.Output, "output", "./dist", "Output directory for .whl files")
	flag.StringVar(&cfg.PyVersion, "py-version", "", "Python package version (default: mirrors -version)")
	platformsFlag := flag.String("platforms", "", "Comma-separated platform keys (default: all)")
	assetsFlag := flag.String("assets", "", "Comma-separated asset filenames to download (overrides auto-detect)")

	// PyPI upload
	flag.BoolVar(&cfg.Upload, "upload", false, "Upload built wheels to PyPI")
	flag.StringVar(&cfg.PyPIURL, "pypi-url", defaultPyPIURL, "PyPI upload endpoint")
	flag.StringVar(&cfg.PyPIUser, "pypi-user", "__token__", "PyPI username")

	// Input files
	flag.StringVar(&cfg.LicensePath, "license", "", "Path to license file (default: fetch from repo)")
	flag.StringVar(&cfg.DescriptionPath, "description", "DESCRIPTION.md", "Path to Markdown description file")

	// Cache & logging
	flag.StringVar(&cfg.CacheDir, "cache", defaultCacheDir(), `Binary cache directory ("" to disable)`)
	flag.BoolVar(&cfg.Debug, "debug", false, "Enable debug-level logging")

	flag.Parse()
	setupLogging(cfg.Debug)

	// Validate -repo
	if cfg.Repo == "" {
		fmt.Fprintln(os.Stderr, "error: -repo is required (e.g. -repo neo4j/mcp)")
		flag.Usage()
		os.Exit(1)
	}
	if !strings.Contains(cfg.Repo, "/") {
		fmt.Fprintln(os.Stderr, "error: -repo must be in owner/name format (e.g. neo4j/mcp)")
		os.Exit(1)
	}

	// Derive defaults from the repo name component.
	repoName := strings.SplitN(cfg.Repo, "/", 2)[1]
	if cfg.BinaryName == "" {
		cfg.BinaryName = repoName
	}
	if cfg.PackageName == "" {
		cfg.PackageName = cfg.BinaryName
	}
	if cfg.EntryPoint == "" {
		cfg.EntryPoint = cfg.BinaryName
	}
	if cfg.Summary == "" {
		cfg.Summary = fmt.Sprintf("%s — packaged as a Python wheel", cfg.PackageName)
	}

	// Parse comma-separated list flags.
	if *platformsFlag != "" {
		for _, k := range strings.Split(*platformsFlag, ",") {
			if k = strings.TrimSpace(k); k != "" {
				cfg.Platforms = append(cfg.Platforms, k)
			}
		}
	}
	if *assetsFlag != "" {
		for _, a := range strings.Split(*assetsFlag, ",") {
			if a = strings.TrimSpace(a); a != "" {
				cfg.AssetNames = append(cfg.AssetNames, a)
			}
		}
	}

	if err := run(cfg); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

// run executes the full build (and optional upload) pipeline.
// It is separated from main so that it can be invoked in tests.
func run(cfg *Config) error {
	if err := os.MkdirAll(cfg.Output, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", cfg.Output, err)
	}

	// Log resolved config so mismatched defaults are immediately visible.
	slog.Info("config",
		"repo", cfg.Repo,
		"binary_name", cfg.BinaryName,
		"package_name", cfg.PackageName,
		"entry_point", cfg.EntryPoint,
	)

	// Resolve PyPI credentials early to fail fast before any network work.
	var pypiPassword string
	if cfg.Upload {
		pypiPassword = os.Getenv("PYPI_TOKEN")
		if pypiPassword == "" {
			pypiPassword = os.Getenv("PYPI_PASSWORD")
		}
		if pypiPassword == "" {
			return fmt.Errorf("-upload requires PYPI_TOKEN (or PYPI_PASSWORD) env var")
		}
	}

	licenseData, err := resolveLicense(cfg)
	if err != nil {
		return fmt.Errorf("license: %w", err)
	}

	descriptionData, err := resolveDescription(cfg.DescriptionPath)
	if err != nil {
		return fmt.Errorf("description: %w", err)
	}

	rel, err := fetchRelease(cfg.Repo, cfg.Version)
	if err != nil {
		return fmt.Errorf("fetch release: %w", err)
	}

	binaryVersion := strings.TrimPrefix(rel.TagName, "v")
	pyVersion := cfg.PyVersion
	if pyVersion == "" {
		pyVersion = binaryVersion
	}
	slog.Info("resolved release",
		"binary_version", binaryVersion,
		"py_version", pyVersion,
	)

	// Log available asset names at debug so mismatches are immediately obvious.
	if slog.Default().Enabled(nil, slog.LevelDebug) {
		names := make([]string, len(rel.Assets))
		for i, a := range rel.Assets {
			names[i] = a.Name
		}
		slog.Debug("release assets available", "count", len(names), "assets", names)
	}

	// Decide which assets to process.
	var assetURLs []assetEntry
	if len(cfg.AssetNames) > 0 {
		assetURLs = resolveAssetsByName(rel.Assets, cfg.AssetNames)
	} else {
		assetURLs = resolveAssetsByPlatform(rel.Assets, cfg.BinaryName, binaryVersion, cfg.Platforms)
	}

	if len(assetURLs) == 0 {
		slog.Warn("no matching assets found in release", "tag", rel.TagName)
	}

	var built []string
	for _, ae := range assetURLs {
		slog.Info("building wheel",
			"platform", ae.PlatformKey,
			"wheel_tag", ae.WheelTag,
			"asset", ae.AssetName,
		)

		cacheDir := ""
		if cfg.CacheDir != "" {
			cacheDir = filepath.Join(cfg.CacheDir, binaryVersion)
		}

		archiveData, err := cachedDownload(ae.URL, cacheDir)
		if err != nil {
			slog.Error("download failed", "asset", ae.AssetName, "error", err)
			continue
		}

		binaryData, err := extractBinary(archiveData, ae.ArchiveExt, ae.BinaryInArc)
		if err != nil {
			slog.Error("extraction failed", "asset", ae.AssetName, "error", err)
			continue
		}

		outPath, err := buildWheel(
			binaryData, ae.BinaryInArc, binaryVersion,
			cfg, pyVersion, ae.WheelTag,
			descriptionData, licenseData,
		)
		if err != nil {
			slog.Error("wheel build failed", "platform", ae.PlatformKey, "error", err)
			continue
		}
		slog.Info("wheel built", "file", filepath.Base(outPath))

		if cfg.Upload {
			slog.Info("uploading wheel", "file", filepath.Base(outPath), "pypi_url", cfg.PyPIURL)
			if err := uploadToPyPI(outPath, cfg.PackageName, pyVersion, cfg.PyPIURL, cfg.PyPIUser, pypiPassword); err != nil {
				slog.Error("upload failed", "file", filepath.Base(outPath), "error", err)
				continue
			}
			slog.Info("wheel uploaded", "file", filepath.Base(outPath))
		}

		built = append(built, outPath)
	}

	slog.Info("done", "wheels_built", len(built), "output_dir", cfg.Output)
	return nil
}
