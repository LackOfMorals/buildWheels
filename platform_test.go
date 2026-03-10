// platform_test.go
package main

import (
	"testing"
)

// assetList is a test helper that builds a []ghAsset from filenames,
// using "https://example.com/<name>" as the download URL.
func assetList(names ...string) []ghAsset {
	out := make([]ghAsset, len(names))
	for i, n := range names {
		out[i] = ghAsset{Name: n, BrowserDownloadURL: "https://example.com/" + n}
	}
	return out
}

// --- resolveAssetsByPlatform ---

func TestResolveAssets_PrimaryPattern(t *testing.T) {
	assets := assetList(
		"mytool_1.2.3_Linux_x86_64.tar.gz",
		"mytool_1.2.3_Darwin_arm64.tar.gz",
	)
	result := resolveAssetsByPlatform(assets, "mytool", "1.2.3", []string{"Linux_x86_64", "Darwin_arm64"})
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	for _, e := range result {
		if e.ArchiveExt != "tar.gz" {
			t.Errorf("%s: expected tar.gz, got %s", e.PlatformKey, e.ArchiveExt)
		}
		if e.BinaryInArc != "mytool" {
			t.Errorf("%s: expected binary 'mytool', got %q", e.PlatformKey, e.BinaryInArc)
		}
		if e.URL == "" {
			t.Errorf("%s: URL is empty", e.PlatformKey)
		}
	}
}

func TestResolveAssets_FallbackPattern(t *testing.T) {
	// No version in name — should fall back to the no-version pattern.
	assets := assetList("mytool_Linux_x86_64.tar.gz")
	result := resolveAssetsByPlatform(assets, "mytool", "1.0.0", []string{"Linux_x86_64"})
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].AssetName != "mytool_Linux_x86_64.tar.gz" {
		t.Errorf("unexpected asset name: %s", result[0].AssetName)
	}
}

func TestResolveAssets_PlatformFilter(t *testing.T) {
	assets := assetList(
		"mytool_1.0.0_Linux_x86_64.tar.gz",
		"mytool_1.0.0_Darwin_arm64.tar.gz",
		"mytool_1.0.0_Windows_x86_64.zip",
	)
	result := resolveAssetsByPlatform(assets, "mytool", "1.0.0", []string{"Linux_x86_64"})
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].PlatformKey != "Linux_x86_64" {
		t.Errorf("unexpected platform: %s", result[0].PlatformKey)
	}
}

func TestResolveAssets_AllPlatformsWhenFilterEmpty(t *testing.T) {
	var names []string
	for k, def := range knownPlatforms {
		names = append(names, "mytool_1.0.0_"+k+"."+def.archiveExt)
	}
	assets := assetList(names...)
	result := resolveAssetsByPlatform(assets, "mytool", "1.0.0", nil)
	if len(result) != len(knownPlatforms) {
		t.Errorf("expected %d entries for all platforms, got %d", len(knownPlatforms), len(result))
	}
}

func TestResolveAssets_WindowsBinaryHasExeSuffix(t *testing.T) {
	assets := assetList("mytool_1.0.0_Windows_x86_64.zip")
	result := resolveAssetsByPlatform(assets, "mytool", "1.0.0", []string{"Windows_x86_64"})
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].BinaryInArc != "mytool.exe" {
		t.Errorf("expected mytool.exe, got %q", result[0].BinaryInArc)
	}
	if result[0].ArchiveExt != "zip" {
		t.Errorf("expected zip, got %q", result[0].ArchiveExt)
	}
}

func TestResolveAssets_NotFound(t *testing.T) {
	assets := assetList("other_1.0.0_Linux_x86_64.tar.gz")
	result := resolveAssetsByPlatform(assets, "mytool", "1.0.0", []string{"Linux_x86_64"})
	if len(result) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result))
	}
}

func TestResolveAssets_WheelTagSet(t *testing.T) {
	assets := assetList("mytool_1.0.0_Linux_x86_64.tar.gz")
	result := resolveAssetsByPlatform(assets, "mytool", "1.0.0", []string{"Linux_x86_64"})
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].WheelTag != "manylinux_2_17_x86_64" {
		t.Errorf("unexpected wheel tag: %s", result[0].WheelTag)
	}
}

// --- resolveAssetsByName ---

func TestResolveAssetsByName_Explicit(t *testing.T) {
	assets := assetList(
		"mytool_1.0.0_Linux_x86_64.tar.gz",
		"mytool_1.0.0_Windows_x86_64.zip",
	)
	result := resolveAssetsByName(assets, []string{"mytool_1.0.0_Linux_x86_64.tar.gz"})
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].ArchiveExt != "tar.gz" {
		t.Errorf("expected tar.gz, got %q", result[0].ArchiveExt)
	}
}

func TestResolveAssetsByName_NotFound(t *testing.T) {
	assets := assetList("mytool_1.0.0_Linux_x86_64.tar.gz")
	result := resolveAssetsByName(assets, []string{"nonexistent.tar.gz"})
	if len(result) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result))
	}
}

func TestResolveAssetsByName_MultipleAssets(t *testing.T) {
	assets := assetList(
		"mytool_1.0.0_Linux_x86_64.tar.gz",
		"mytool_1.0.0_Darwin_arm64.tar.gz",
	)
	result := resolveAssetsByName(assets, []string{
		"mytool_1.0.0_Linux_x86_64.tar.gz",
		"mytool_1.0.0_Darwin_arm64.tar.gz",
	})
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
}

// --- detectArchiveExt ---

func TestDetectArchiveExt(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"foo.tar.gz", "tar.gz"},
		{"foo.zip", "zip"},
		{"foo.exe", ""},
		{"foo", ""},
		{"foo.tar", ""},
		{"foo.gz", ""},
	}
	for _, tt := range tests {
		got := detectArchiveExt(tt.name)
		if got != tt.want {
			t.Errorf("detectArchiveExt(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// --- inferPlatform ---

func TestInferPlatform(t *testing.T) {
	tests := []struct {
		filename string
		wantKey  string
		wantTag  string
	}{
		{"tool_1.0_linux_x86_64.tar.gz", "Linux_x86_64", "manylinux_2_17_x86_64"},
		{"tool_1.0_darwin_arm64.tar.gz", "Darwin_arm64", "macosx_11_0_arm64"},
		{"tool_1.0_windows_x86_64.zip", "Windows_x86_64", "win_amd64"},
		{"tool_1.0_linux_arm64.tar.gz", "Linux_arm64", "manylinux_2_17_aarch64"},
		{"tool_1.0_darwin_x86_64.tar.gz", "Darwin_x86_64", "macosx_10_9_x86_64"},
		{"tool_1.0_windows_arm64.zip", "Windows_arm64", "win_arm64"},
		{"completely_unknown.tar.gz", "unknown", "any"},
	}
	for _, tt := range tests {
		platKey, wheelTag := inferPlatform(tt.filename)
		if platKey != tt.wantKey {
			t.Errorf("inferPlatform(%q) platKey = %q, want %q", tt.filename, platKey, tt.wantKey)
		}
		if wheelTag != tt.wantTag {
			t.Errorf("inferPlatform(%q) wheelTag = %q, want %q", tt.filename, wheelTag, tt.wantTag)
		}
	}
}

// --- buildWantedSet ---

func TestBuildWantedSet_NilMeansAll(t *testing.T) {
	s := buildWantedSet(nil)
	if len(s) != len(knownPlatforms) {
		t.Errorf("nil input should produce a set with all %d platforms, got %d",
			len(knownPlatforms), len(s))
	}
}

func TestBuildWantedSet_Specific(t *testing.T) {
	s := buildWantedSet([]string{"Linux_x86_64", "Darwin_arm64"})
	if !s["Linux_x86_64"] {
		t.Error("Linux_x86_64 should be in the set")
	}
	if !s["Darwin_arm64"] {
		t.Error("Darwin_arm64 should be in the set")
	}
	if s["Windows_x86_64"] {
		t.Error("Windows_x86_64 should NOT be in the set")
	}
}

// --- indexAssets ---

func TestIndexAssets(t *testing.T) {
	assets := []ghAsset{
		{Name: "a.tar.gz", BrowserDownloadURL: "https://example.com/a.tar.gz"},
		{Name: "b.zip", BrowserDownloadURL: "https://example.com/b.zip"},
	}
	idx := indexAssets(assets)
	if idx["a.tar.gz"] != "https://example.com/a.tar.gz" {
		t.Error("index lookup for a.tar.gz failed")
	}
	if idx["b.zip"] != "https://example.com/b.zip" {
		t.Error("index lookup for b.zip failed")
	}
	if _, ok := idx["missing"]; ok {
		t.Error("unexpected key 'missing' in index")
	}
}
