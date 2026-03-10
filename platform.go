// platform.go — mapping between GoReleaser OS/Arch keys and Python wheel
// platform tags, plus release-asset resolution logic.
package main

import (
	"fmt"
	"log/slog"
	"path"
	"strings"
)

// platformDef maps a GoReleaser OS_Arch key to the Python wheel platform tag
// and archive format used for that target.
type platformDef struct {
	wheelTag   string // Python wheel platform tag
	archiveExt string // "tar.gz" or "zip"
	windows    bool   // whether the binary gets an .exe suffix
}

// knownPlatforms is the canonical set of supported build targets.
var knownPlatforms = map[string]platformDef{
	"Darwin_x86_64":  {"macosx_10_9_x86_64", "tar.gz", false},
	"Darwin_arm64":   {"macosx_11_0_arm64", "tar.gz", false},
	"Linux_x86_64":   {"manylinux_2_17_x86_64", "tar.gz", false},
	"Linux_arm64":    {"manylinux_2_17_aarch64", "tar.gz", false},
	"Windows_x86_64": {"win_amd64", "zip", true},
	"Windows_arm64":  {"win_arm64", "zip", true},
}

// assetEntry is a release asset fully resolved and ready to download.
type assetEntry struct {
	PlatformKey string // GoReleaser OS_Arch key, e.g. "Linux_x86_64"
	WheelTag    string // Python wheel platform tag
	ArchiveExt  string // "tar.gz" or "zip"
	BinaryInArc string // filename of the binary inside the archive
	AssetName   string // GitHub release asset filename
	URL         string // download URL
}

// resolveAssetsByPlatform matches release assets against knownPlatforms using
// GoReleaser's default naming conventions.
//
// Patterns tried per platform (first match wins):
//  1. {binary}_{version}_{OS_Arch}.{ext}
//  2. {binary}_{OS_Arch}.{ext}  (no-version fallback)
//
// wantPlatforms filters by GoReleaser key; pass nil for all platforms.
func resolveAssetsByPlatform(assets []ghAsset, binaryName, version string, wantPlatforms []string) []assetEntry {
	wanted := buildWantedSet(wantPlatforms)
	idx := indexAssets(assets)

	var result []assetEntry
	for platKey, def := range knownPlatforms {
		if !wanted[platKey] {
			continue
		}

		binInArc := binaryName
		if def.windows {
			binInArc = binaryName + ".exe"
		}

		// Primary pattern: {binary}_{version}_{OS_Arch}.{ext}
		primary := fmt.Sprintf("%s_%s_%s.%s", binaryName, version, platKey, def.archiveExt)
		// Fallback pattern: {binary}_{OS_Arch}.{ext}  (no version in filename)
		fallback := fmt.Sprintf("%s_%s.%s", binaryName, platKey, def.archiveExt)

		slog.Debug("trying asset names", "platform", platKey, "primary", primary, "fallback", fallback)

		assetName := primary
		url, ok := idx[primary]
		if !ok {
			assetName = fallback
			url, ok = idx[fallback]
		}
		if !ok {
			slog.Debug("no asset found for platform, skipping",
				"platform", platKey,
				"tried_primary", primary,
				"tried_fallback", fallback,
			)
			continue
		}

		result = append(result, assetEntry{
			PlatformKey: platKey,
			WheelTag:    def.wheelTag,
			ArchiveExt:  def.archiveExt,
			BinaryInArc: binInArc,
			AssetName:   assetName,
			URL:         url,
		})
	}
	return result
}

// resolveAssetsByName resolves a caller-specified list of asset filenames,
// inferring platform metadata from the filename where possible. This is the
// path taken when -assets is supplied on the CLI.
func resolveAssetsByName(assets []ghAsset, assetNames []string) []assetEntry {
	idx := indexAssets(assets)

	var result []assetEntry
	for _, name := range assetNames {
		url, ok := idx[name]
		if !ok {
			slog.Warn("specified asset not found in release, skipping", "asset", name)
			continue
		}

		ext := detectArchiveExt(name)
		platKey, wheelTag := inferPlatform(name)

		// Binary name is the first underscore-delimited segment of the filename.
		binBase := strings.SplitN(path.Base(name), "_", 2)[0]
		binInArc := binBase
		if strings.Contains(strings.ToLower(name), "windows") {
			binInArc = binBase + ".exe"
		}

		result = append(result, assetEntry{
			PlatformKey: platKey,
			WheelTag:    wheelTag,
			ArchiveExt:  ext,
			BinaryInArc: binInArc,
			AssetName:   name,
			URL:         url,
		})
	}
	return result
}

// detectArchiveExt returns "tar.gz", "zip", or "" based on the filename suffix.
func detectArchiveExt(name string) string {
	switch {
	case strings.HasSuffix(name, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(name, ".zip"):
		return "zip"
	default:
		return ""
	}
}

// inferPlatform returns the best-matching GoReleaser platform key and Python
// wheel tag for an asset filename that follows GoReleaser conventions.
// Returns ("unknown", "any") when no match is found.
func inferPlatform(name string) (platKey, wheelTag string) {
	lower := strings.ToLower(name)
	for k, def := range knownPlatforms {
		parts := strings.SplitN(strings.ToLower(k), "_", 2)
		if len(parts) == 2 &&
			strings.Contains(lower, parts[0]) &&
			strings.Contains(lower, parts[1]) {
			return k, def.wheelTag
		}
	}
	return "unknown", "any"
}

// buildWantedSet converts a platform filter slice into a lookup set.
// An empty slice means "all known platforms".
func buildWantedSet(platforms []string) map[string]bool {
	s := make(map[string]bool, len(knownPlatforms))
	if len(platforms) == 0 {
		for k := range knownPlatforms {
			s[k] = true
		}
		return s
	}
	for _, k := range platforms {
		s[k] = true
	}
	return s
}

// indexAssets builds a name→URL map from a slice of ghAsset for O(1) lookup.
func indexAssets(assets []ghAsset) map[string]string {
	m := make(map[string]string, len(assets))
	for _, a := range assets {
		m[a.Name] = a.BrowserDownloadURL
	}
	return m
}
