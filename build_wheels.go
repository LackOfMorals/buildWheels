// build_wheels.go
// Builds (and optionally uploads) Python wheels for neo4j-mcp using
// pre-built binaries from https://github.com/neo4j/mcp/releases
//
// Usage:
//   go run build_wheels.go [flags]
//
// Flags:
//   -version      MCP server release tag, e.g. v1.4.2      (default: latest)
//   -py-version   Python package version, e.g. 1.4.2.1     (default: mirrors -version)
//   -output       output directory                          (default: ./dist)
//   -platforms    comma-separated platform keys             (default: all)
//   -upload       upload wheels to PyPI                     (default: false)
//   -pypi-url     PyPI upload endpoint                      (default: https://upload.pypi.org/legacy/)
//   -pypi-user    PyPI username                             (default: __token__)
//   -license      path to license file                      (default: fetched from neo4j/mcp)
//   -description  path to Markdown description file         (default: DESCRIPTION.md)
//   -cache        directory to cache downloaded binaries    (default: OS cache dir)
//
// Environment variables:
//   PYPI_TOKEN    PyPI API token (required when -upload is set)
//   PYPI_PASSWORD alternative to PYPI_TOKEN
//   GITHUB_TOKEN  GitHub PAT to avoid API rate limits

package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/md5" //nolint:gosec // MD5 required by PyPI upload API
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const (
	githubRepo     = "neo4j/mcp"
	binaryName     = "neo4j-mcp"
	packageName    = "neo4j-mcp"
	entryPoint     = "neo4j-mcp"
	defaultPyPIURL = "https://upload.pypi.org/legacy/"
)

type platform struct {
	wheelTag    string // Python wheel platform tag
	archiveExt  string // "tar.gz" or "zip"
	binaryInArc string // filename inside the archive
}

var platformMap = map[string]platform{
	"Darwin_amd64":  {"macosx_10_9_x86_64", "tar.gz", binaryName},
	"Darwin_arm64":  {"macosx_11_0_arm64", "tar.gz", binaryName},
	"Linux_amd64":   {"manylinux_2_17_x86_64", "tar.gz", binaryName},
	"Linux_arm64":   {"manylinux_2_17_aarch64", "tar.gz", binaryName},
	"Windows_amd64": {"win_amd64", "zip", binaryName + ".exe"},
	"Windows_arm64": {"win_arm64", "zip", binaryName + ".exe"},
}

// ---------------------------------------------------------------------------
// GitHub API
// ---------------------------------------------------------------------------

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

func ghGet(urlPath string) ([]byte, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", githubRepo, urlPath)
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

func fetchRelease(tag string) (ghRelease, error) {
	var rel ghRelease
	var (
		data []byte
		err  error
	)
	if tag == "" {
		fmt.Println("Fetching latest release …")
		data, err = ghGet("releases/latest")
	} else {
		data, err = ghGet("releases/tags/" + tag)
	}
	if err != nil {
		return rel, err
	}
	return rel, json.Unmarshal(data, &rel)
}

// ---------------------------------------------------------------------------
// Download with cache
// ---------------------------------------------------------------------------

func downloadBytes(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// cachedDownload returns the bytes for url, reading from cacheDir/<filename>
// if present, or downloading and storing it there if not.
// If cacheDir is empty, the download is always performed with no caching.
func cachedDownload(url, cacheDir string) ([]byte, error) {
	filename := path.Base(url)

	if cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return nil, fmt.Errorf("create cache dir: %w", err)
		}
		cachePath := filepath.Join(cacheDir, filename)
		if data, err := os.ReadFile(cachePath); err == nil {
			fmt.Printf("  ✓ cache hit  %s\n", filename)
			return data, nil
		}
		// Cache miss — download then store
		fmt.Printf("  ↓ %s\n", url)
		data, err := downloadBytes(url)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(cachePath, data, 0o644); err != nil {
			// Non-fatal: warn but still return the data
			fmt.Printf("  ⚠ could not write cache file %s: %v\n", cachePath, err)
		} else {
			fmt.Printf("  ✓ cached to  %s\n", cachePath)
		}
		return data, nil
	}

	fmt.Printf("  ↓ %s\n", url)
	return downloadBytes(url)
}

// ---------------------------------------------------------------------------
// Archive extraction
// ---------------------------------------------------------------------------

func extractFromTarGz(data []byte, target string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if path.Base(hdr.Name) == target {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%q not found in tar.gz archive", target)
}

func extractFromZip(data []byte, target string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if path.Base(f.Name) == target {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%q not found in zip archive", target)
}

func extractBinary(archiveData []byte, ext, binaryFilename string) ([]byte, error) {
	switch ext {
	case "tar.gz":
		return extractFromTarGz(archiveData, binaryFilename)
	case "zip":
		return extractFromZip(archiveData, binaryFilename)
	default:
		return nil, fmt.Errorf("unknown archive type: %s", ext)
	}
}

// ---------------------------------------------------------------------------
// License & description
// ---------------------------------------------------------------------------

// resolveLicense returns the license file contents. If licensePath is
// non-empty it reads from disk; otherwise it fetches LICENSE.txt from the
// neo4j/mcp repo.
func resolveLicense(licensePath string) ([]byte, error) {
	if licensePath != "" {
		data, err := os.ReadFile(licensePath)
		if err != nil {
			return nil, fmt.Errorf("reading license file %s: %w", licensePath, err)
		}
		fmt.Printf("Using license from %s\n", licensePath)
		return data, nil
	}

	url := "https://raw.githubusercontent.com/neo4j/mcp/main/LICENSE.txt"
	fmt.Printf("Fetching license from %s …\n", url)
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("fetching license: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching license: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// resolveDescription reads the long description from a local Markdown file.
func resolveDescription(descPath string) ([]byte, error) {
	if descPath == "" {
		descPath = "DESCRIPTION.md"
	}
	data, err := os.ReadFile(descPath)
	if err != nil {
		return nil, fmt.Errorf("reading description file %s: %w", descPath, err)
	}
	fmt.Printf("Using description from %s\n", descPath)
	return data, nil
}

// ---------------------------------------------------------------------------
// Wheel construction
// ---------------------------------------------------------------------------

func normalize(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

func wheelFilename(pkg, version, plat string) string {
	return fmt.Sprintf("%s-%s-py3-none-%s.whl", normalize(pkg), version, plat)
}

// unixShim uses os.execv to replace the current process — zero subprocess overhead.
const unixShim = `import os, sys

def main():
    here = os.path.dirname(os.path.abspath(__file__))
    binary = os.path.join(here, %q)
    os.execv(binary, [binary] + sys.argv[1:])
`

// windowsShim falls back to subprocess because execv is not reliable on Windows.
const windowsShim = `import os, sys, subprocess

def main():
    here = os.path.dirname(os.path.abspath(__file__))
    binary = os.path.join(here, %q)
    sys.exit(subprocess.call([binary] + sys.argv[1:]))
`

// recordHash returns the base64url-encoded (no padding) SHA-256 digest
// formatted as required by the wheel RECORD spec: "sha256=<digest>".
func recordHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256=" + base64.RawURLEncoding.EncodeToString(sum[:])
}

func buildWheel(
	binaryData []byte,
	binaryFilename, binVer, pkg, pyVersion, plat, outputDir string,
	licenseData, descriptionData []byte,
) (string, error) {
	pkgNorm := normalize(pkg)
	isWindows := strings.HasSuffix(binaryFilename, ".exe")

	var shimSrc string
	if isWindows {
		shimSrc = fmt.Sprintf(windowsShim, binaryFilename)
	} else {
		shimSrc = fmt.Sprintf(unixShim, binaryFilename)
	}

	initSrc := fmt.Sprintf("# %s — generated shim package\n__version__ = %q\n", pkg, pyVersion)
	distInfo := fmt.Sprintf("%s-%s.dist-info", pkgNorm, pyVersion)

	// Metadata 2.4: long description goes in the message body, separated
	// from the headers by a single blank line (RFC 822 convention).
	metadata := fmt.Sprintf(
		"Metadata-Version: 2.4\n"+
			"Name: %s\n"+
			"Version: %s\n"+
			"Summary: Neo4j official MCP Server version %s — packaged as a Python wheel\n"+
			"Project-URL: Source, https://github.com/neo4j/mcp\n"+
			"Classifier: Programming Language :: Python :: 3\n"+
			"License-Expression: GPL-3.0-or-later\n"+
			"License-File: LICENSE.txt\n"+
			"Requires-Python: >=3.9\n"+
			"Keywords: mcp,neo4j\n"+
			"Description-Content-Type: text/markdown; charset=UTF-8; variant=GFM\n"+
			"\n"+
			"%s",
		pkg, pyVersion, binVer, string(descriptionData))

	wheelMeta := fmt.Sprintf(
		"Wheel-Version: 1.0\nGenerator: build_wheels.go\nRoot-Is-Purelib: false\nTag: py3-none-%s\n",
		plat)

	entryPoints := fmt.Sprintf("[console_scripts]\n%s = %s._shim:main\n", entryPoint, pkgNorm)

	// --- Pass 1: collect all entries so we can build a proper RECORD ---
	type entry struct {
		name string
		data []byte
		exe  bool
	}
	entries := []entry{
		{pkgNorm + "/" + binaryFilename, binaryData, true},
		{pkgNorm + "/__init__.py", []byte(initSrc), false},
		{pkgNorm + "/_shim.py", []byte(shimSrc), false},
		{distInfo + "/METADATA", []byte(metadata), false},
		{distInfo + "/WHEEL", []byte(wheelMeta), false},
		{distInfo + "/entry_points.txt", []byte(entryPoints), false},
		{distInfo + "/licenses/LICENSE.txt", licenseData, false},
	}

	// RECORD: one CSV line per file (path,hash,size), then the RECORD entry
	// itself with empty hash and size as required by the spec.
	var rec strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&rec, "%s,%s,%d\n", e.name, recordHash(e.data), len(e.data))
	}
	recordName := distInfo + "/RECORD"
	fmt.Fprintf(&rec, "%s,,\n", recordName)

	// --- Pass 2: write zip ---
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	addEntry := func(name string, data []byte, exe bool) error {
		// Set both 32-bit and 64-bit size fields.
		// If only CompressedSize64 is non-zero (CompressedSize=0), Go writes
		// zip64 extra fields even for small files. uv's zip reader skips
		// entries with unexpected zip64 fields, producing "WHEEL not found".
		// Populating the 32-bit fields keeps Go in standard zip32 format.
		// Setting Flags=0 explicitly prevents the data descriptor flag (bit 3)
		// which causes PyPI/twine to reject the upload.
		size := uint64(len(data))
		size32 := uint32(size)
		fh := &zip.FileHeader{
			Name:               name,
			Method:             zip.Store,
			Flags:              0,
			Modified:           time.Now(),
			CRC32:              crc32.ChecksumIEEE(data),
			CompressedSize:     size32,
			UncompressedSize:   size32,
			CompressedSize64:   size,
			UncompressedSize64: size,
		}
		if exe {
			fh.SetMode(0o755)
		} else {
			fh.SetMode(0o644)
		}
		w, err := zw.CreateRaw(fh)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}

	for _, e := range entries {
		if err := addEntry(e.name, e.data, e.exe); err != nil {
			return "", fmt.Errorf("adding %s to wheel: %w", e.name, err)
		}
	}
	// RECORD must be last
	if err := addEntry(recordName, []byte(rec.String()), false); err != nil {
		return "", fmt.Errorf("adding RECORD to wheel: %w", err)
	}

	if err := zw.Close(); err != nil {
		return "", err
	}

	out := filepath.Join(outputDir, wheelFilename(pkg, pyVersion, plat))
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// PyPI upload
// ---------------------------------------------------------------------------

// wheelDigests returns the MD5 and SHA-256 hex digests of the wheel bytes.
func wheelDigests(data []byte) (md5hex, sha256hex string) {
	m := md5.Sum(data) //nolint:gosec // required by PyPI legacy API
	s := sha256.Sum256(data)
	return fmt.Sprintf("%x", m), fmt.Sprintf("%x", s)
}

// uploadToPyPI uploads a single wheel to the PyPI legacy upload endpoint.
// username is typically "__token__" when using an API token.
func uploadToPyPI(wheelPath, pkg, version, pypiURL, username, password string) error {
	wheelData, err := os.ReadFile(wheelPath)
	if err != nil {
		return fmt.Errorf("read wheel: %w", err)
	}

	md5hex, sha256hex := wheelDigests(wheelData)
	filename := filepath.Base(wheelPath)

	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)

	fields := map[string]string{
		":action":          "file_upload",
		"protocol_version": "1",
		"filetype":         "bdist_wheel",
		"pyversion":        "py3",
		"metadata_version": "2.4",
		"name":             pkg,
		"version":          version,
		"md5_digest":       md5hex,
		"sha2_digest":      sha256hex,
	}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return err
		}
	}

	// Attach the wheel file with the correct MIME type
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="content"; filename=%q`, filename))
	h.Set("Content-Type", "application/zip")
	fw, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	if _, err = fw.Write(wheelData); err != nil {
		return err
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPost, pypiURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.SetBasicAuth(username, password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest:
		msg := string(respBody)
		if strings.Contains(msg, "already exists") || strings.Contains(msg, "File already") {
			fmt.Printf("  ⚠ already exists on PyPI, skipping\n")
			return nil
		}
		return fmt.Errorf("PyPI 400: %s", msg)
	default:
		return fmt.Errorf("PyPI %s: %s", resp.Status, string(respBody))
	}
}

// ---------------------------------------------------------------------------
// Cache directory
// ---------------------------------------------------------------------------

// defaultCacheDir returns a sensible OS-specific cache location:
//   - Linux:   $XDG_CACHE_HOME/neo4j-mcp-wheels  (~/.cache/neo4j-mcp-wheels)
//   - macOS:   ~/Library/Caches/neo4j-mcp-wheels
//   - Windows: %LocalAppData%\neo4j-mcp-wheels
//
// Falls back to ./.cache if the OS cache dir cannot be determined.
func defaultCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "neo4j-mcp-wheels")
	}
	return ".cache"
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	versionFlag := flag.String("version", "", "MCP server release tag to download, e.g. v1.4.2 (default: latest)")
	pyVersionFlag := flag.String("py-version", "", "Python package version, e.g. 1.4.2.1 (default: mirrors -version)")
	outputFlag := flag.String("output", "./dist", "Output directory for .whl files")
	platformsFlag := flag.String("platforms", "", "Comma-separated platform keys; default: all")
	uploadFlag := flag.Bool("upload", false, "Upload built wheels to PyPI")
	pypiURLFlag := flag.String("pypi-url", defaultPyPIURL, "PyPI upload endpoint")
	pypiUserFlag := flag.String("pypi-user", "__token__", "PyPI username (use __token__ for API tokens)")
	licenseFlag := flag.String("license", "", "Path to a license file; defaults to fetching LICENSE.txt from neo4j/mcp")
	descriptionFlag := flag.String("description", "DESCRIPTION.md", "Path to a Markdown description file")
	cacheFlag := flag.String("cache", defaultCacheDir(), "Directory to cache downloaded binaries; set to \"\" to disable")
	flag.Parse()

	if err := os.MkdirAll(*outputFlag, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", *outputFlag, err)
		os.Exit(1)
	}

	// Resolve PyPI password early so we fail fast before doing any work
	var pypiPassword string
	if *uploadFlag {
		pypiPassword = os.Getenv("PYPI_TOKEN")
		if pypiPassword == "" {
			pypiPassword = os.Getenv("PYPI_PASSWORD")
		}
		if pypiPassword == "" {
			fmt.Fprintln(os.Stderr, "error: -upload requires PYPI_TOKEN (or PYPI_PASSWORD) env var")
			os.Exit(1)
		}
	}

	// Resolve license content
	licenseData, err := resolveLicense(*licenseFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "license: %v\n", err)
		os.Exit(1)
	}

	// Resolve description content
	descriptionData, err := resolveDescription(*descriptionFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "description: %v\n", err)
		os.Exit(1)
	}

	// Resolve which platforms to build
	wanted := map[string]bool{}
	if *platformsFlag == "" {
		for k := range platformMap {
			wanted[k] = true
		}
	} else {
		for _, k := range strings.Split(*platformsFlag, ",") {
			wanted[strings.TrimSpace(k)] = true
		}
	}

	// Fetch release metadata from GitHub
	rel, err := fetchRelease(*versionFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch release: %v\n", err)
		os.Exit(1)
	}

	binaryVersion := strings.TrimPrefix(rel.TagName, "v")

	// Python package version defaults to the binary version but can be
	// overridden, e.g. to add a post-release suffix like "1.4.2.1".
	pyVersion := *pyVersionFlag
	if pyVersion == "" {
		pyVersion = binaryVersion
	}

	fmt.Printf("MCP binary version : %s\n", binaryVersion)
	fmt.Printf("Python pkg version : %s\n\n", pyVersion)

	// Index assets by name for O(1) lookup
	assets := make(map[string]string, len(rel.Assets))
	for _, a := range rel.Assets {
		assets[a.Name] = a.BrowserDownloadURL
	}

	var built []string
	for platKey, p := range platformMap {
		if !wanted[platKey] {
			continue
		}

		// Goreleaser default naming: neo4j-mcp_1.4.2_linux_amd64.tar.gz
		assetName := fmt.Sprintf("%s_%s_%s.%s", binaryName, binaryVersion, platKey, p.archiveExt)
		url, ok := assets[assetName]
		if !ok {
			// Fallback: without version in name
			assetName = fmt.Sprintf("%s_%s.%s", binaryName, platKey, p.archiveExt)
			url, ok = assets[assetName]
			if !ok {
				fmt.Printf("[SKIP] %s — asset not found (%s)\n\n", platKey, assetName)
				continue
			}
		}

		fmt.Printf("[%s]  →  %s\n", platKey, p.wheelTag)

		cacheDir := ""
		if *cacheFlag != "" {
			cacheDir = filepath.Join(*cacheFlag, binaryVersion)
		}
		archiveData, err := cachedDownload(url, cacheDir)
		if err != nil {
			fmt.Printf("  ERROR downloading: %v\n\n", err)
			continue
		}

		binaryData, err := extractBinary(archiveData, p.archiveExt, p.binaryInArc)
		if err != nil {
			fmt.Printf("  ERROR extracting binary: %v\n\n", err)
			continue
		}

		outPath, err := buildWheel(
			binaryData, p.binaryInArc, binaryVersion,
			packageName, pyVersion, p.wheelTag,
			*outputFlag,
			licenseData, descriptionData,
		)
		if err != nil {
			fmt.Printf("  ERROR building wheel: %v\n\n", err)
			continue
		}
		fmt.Printf("  ✓ %s\n", filepath.Base(outPath))

		if *uploadFlag {
			fmt.Printf("  ↑ uploading to %s …\n", *pypiURLFlag)
			if err := uploadToPyPI(outPath, packageName, pyVersion, *pypiURLFlag, *pypiUserFlag, pypiPassword); err != nil {
				fmt.Printf("  ERROR uploading: %v\n\n", err)
				continue
			}
			fmt.Printf("  ✓ uploaded\n")
		}

		fmt.Println()
		built = append(built, outPath)
	}

	fmt.Printf("Built %d wheel(s) in %s/\n", len(built), *outputFlag)
	for _, w := range built {
		fmt.Printf("  %s\n", filepath.Base(w))
	}
}
