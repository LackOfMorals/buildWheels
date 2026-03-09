// wheel_test.go
package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testCfg returns a minimal Config suitable for wheel-building tests.
func testCfg(t *testing.T) *Config {
	t.Helper()
	return &Config{
		Repo:        "owner/myrepo",
		PackageName: "myrepo",
		EntryPoint:  "myrepo",
		Summary:     "A test package",
		LicenseExpr: "MIT",
		Output:      t.TempDir(),
	}
}

// wheelEntries opens a .whl file and returns the set of entry names it contains.
func wheelEntries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read wheel: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open wheel as zip: %v", err)
	}
	entries := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", f.Name, err)
		}
		entries[f.Name] = b
	}
	return entries
}

func TestBuildWheel_RequiredEntries(t *testing.T) {
	cfg := testCfg(t)
	outPath, err := buildWheel(
		[]byte("fake binary"), "myrepo", "1.2.3",
		cfg, "1.2.3", "manylinux_2_17_x86_64",
		[]byte("# Description"), []byte("MIT License"),
	)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	entries := wheelEntries(t, outPath)

	required := []string{
		"myrepo/myrepo",
		"myrepo/__init__.py",
		"myrepo/_shim.py",
		"myrepo-1.2.3.dist-info/METADATA",
		"myrepo-1.2.3.dist-info/WHEEL",
		"myrepo-1.2.3.dist-info/entry_points.txt",
		"myrepo-1.2.3.dist-info/licenses/LICENSE.txt",
		"myrepo-1.2.3.dist-info/RECORD",
	}
	for _, r := range required {
		if _, ok := entries[r]; !ok {
			t.Errorf("missing wheel entry: %s", r)
		}
	}
}

func TestBuildWheel_Filename(t *testing.T) {
	cfg := testCfg(t)
	outPath, err := buildWheel(
		[]byte("bin"), "myrepo", "2.0.0",
		cfg, "2.0.0", "macosx_11_0_arm64",
		[]byte("desc"), []byte("lic"),
	)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}
	base := filepath.Base(outPath)
	want := "myrepo-2.0.0-py3-none-macosx_11_0_arm64.whl"
	if base != want {
		t.Errorf("filename = %q, want %q", base, want)
	}
}

func TestBuildWheel_UnixShim(t *testing.T) {
	cfg := testCfg(t)
	outPath, err := buildWheel(
		[]byte("bin"), "myrepo", "1.0.0",
		cfg, "1.0.0", "manylinux_2_17_x86_64",
		[]byte("desc"), []byte("lic"),
	)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	entries := wheelEntries(t, outPath)
	shimSrc := string(entries["myrepo/_shim.py"])

	if !strings.Contains(shimSrc, "os.execv") {
		t.Error("unix shim should use os.execv")
	}
	if strings.Contains(shimSrc, "subprocess") {
		t.Error("unix shim should not use subprocess")
	}
}

func TestBuildWheel_WindowsShim(t *testing.T) {
	cfg := testCfg(t)
	outPath, err := buildWheel(
		[]byte("exe data"), "myrepo.exe", "1.0.0",
		cfg, "1.0.0", "win_amd64",
		[]byte("desc"), []byte("lic"),
	)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	entries := wheelEntries(t, outPath)
	shimSrc := string(entries["myrepo/_shim.py"])

	if !strings.Contains(shimSrc, "subprocess") {
		t.Error("windows shim should use subprocess")
	}
	if strings.Contains(shimSrc, "os.execv") {
		t.Error("windows shim should not use os.execv")
	}
}

func TestBuildWheel_MetadataContents(t *testing.T) {
	cfg := testCfg(t)
	cfg.Summary = "My great tool"
	cfg.LicenseExpr = "Apache-2.0"

	outPath, err := buildWheel(
		[]byte("bin"), "myrepo", "3.1.4",
		cfg, "3.1.4", "manylinux_2_17_x86_64",
		[]byte("long description here"), []byte("Apache License"),
	)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	entries := wheelEntries(t, outPath)
	metadata := string(entries["myrepo-3.1.4.dist-info/METADATA"])

	checks := []string{
		"Name: myrepo",
		"Version: 3.1.4",
		"Summary: My great tool",
		"License-Expression: Apache-2.0",
		"Project-URL: Source, https://github.com/owner/myrepo",
		"long description here",
	}
	for _, c := range checks {
		if !strings.Contains(metadata, c) {
			t.Errorf("METADATA missing %q", c)
		}
	}
}

func TestBuildWheel_WheelTag(t *testing.T) {
	cfg := testCfg(t)
	outPath, err := buildWheel(
		[]byte("bin"), "myrepo", "1.0.0",
		cfg, "1.0.0", "win_amd64",
		[]byte("d"), []byte("l"),
	)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	entries := wheelEntries(t, outPath)
	wheelMeta := string(entries["myrepo-1.0.0.dist-info/WHEEL"])

	if !strings.Contains(wheelMeta, "Tag: py3-none-win_amd64") {
		t.Errorf("WHEEL metadata missing expected Tag line, got:\n%s", wheelMeta)
	}
}

func TestBuildWheel_EntryPoints(t *testing.T) {
	cfg := testCfg(t)
	cfg.EntryPoint = "my-cli"

	outPath, err := buildWheel(
		[]byte("bin"), "myrepo", "1.0.0",
		cfg, "1.0.0", "manylinux_2_17_x86_64",
		[]byte("d"), []byte("l"),
	)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	entries := wheelEntries(t, outPath)
	ep := string(entries["myrepo-1.0.0.dist-info/entry_points.txt"])

	if !strings.Contains(ep, "my-cli = myrepo._shim:main") {
		t.Errorf("entry_points.txt missing expected entry, got:\n%s", ep)
	}
}

func TestBuildWheel_RecordPresent(t *testing.T) {
	cfg := testCfg(t)
	outPath, err := buildWheel(
		[]byte("bin"), "myrepo", "1.0.0",
		cfg, "1.0.0", "manylinux_2_17_x86_64",
		[]byte("d"), []byte("l"),
	)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	entries := wheelEntries(t, outPath)
	record := string(entries["myrepo-1.0.0.dist-info/RECORD"])

	// RECORD must reference all other entries.
	for name := range entries {
		if name == "myrepo-1.0.0.dist-info/RECORD" {
			continue // RECORD references itself with empty hash/size
		}
		if !strings.Contains(record, name) {
			t.Errorf("RECORD missing entry for %s", name)
		}
	}
}

func TestBuildWheel_HyphenatedPackage(t *testing.T) {
	cfg := testCfg(t)
	cfg.PackageName = "my-tool"
	cfg.EntryPoint = "my-tool"

	outPath, err := buildWheel(
		[]byte("bin"), "my-tool", "1.0.0",
		cfg, "1.0.0", "manylinux_2_17_x86_64",
		[]byte("d"), []byte("l"),
	)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	base := filepath.Base(outPath)
	if !strings.HasPrefix(base, "my_tool-") {
		t.Errorf("expected normalized filename prefix 'my_tool-', got %q", base)
	}
}

// --- normalize ---

func TestNormalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"my-package", "my_package"},
		{"already_ok", "already_ok"},
		{"a-b-c", "a_b_c"},
		{"nohyphen", "nohyphen"},
	}
	for _, tt := range tests {
		got := normalize(tt.in)
		if got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- wheelFilename ---

func TestWheelFilename(t *testing.T) {
	got := wheelFilename("my-pkg", "1.2.3", "manylinux_2_17_x86_64")
	want := "my_pkg-1.2.3-py3-none-manylinux_2_17_x86_64.whl"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWheelFilename_Py3Tag(t *testing.T) {
	// The filename must use "py3", not "py30".
	got := wheelFilename("pkg", "1.0.0", "win_amd64")
	if !strings.Contains(got, "-py3-") {
		t.Errorf("expected -py3- in filename, got %q", got)
	}
	if strings.Contains(got, "py30") {
		t.Errorf("filename must not contain 'py30', got %q", got)
	}
}

// --- recordHash ---

func TestRecordHash_Format(t *testing.T) {
	h := recordHash([]byte("hello"))
	if !strings.HasPrefix(h, "sha256=") {
		t.Errorf("expected sha256= prefix, got %q", h)
	}
}

func TestRecordHash_KnownValue(t *testing.T) {
	// SHA-256("hello") base64url (no padding) = LPJNul-wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ
	got := recordHash([]byte("hello"))
	want := "sha256=LPJNul-wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRecordHash_Deterministic(t *testing.T) {
	data := []byte("some data")
	h1 := recordHash(data)
	h2 := recordHash(data)
	if h1 != h2 {
		t.Error("recordHash is not deterministic")
	}
}
