// wheel.go — Python wheel (.whl) construction.
//
// A wheel is a zip file with a specific internal layout. This file handles
// building the shim package, metadata, and RECORD, then writing the zip.
//
// Note: the Python compatibility tag is "py3" (any Python 3), consistent with
// the Tag field in the WHEEL metadata. The original build_wheels.go had a
// latent mismatch (filename used "py30"; WHEEL metadata used "py3") which is
// corrected here.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// normalize converts a package name to the underscore form required by the
// wheel filename specification (PEP 427).
func normalize(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// wheelFilename returns the canonical .whl filename for the given package,
// version, and platform tag.
func wheelFilename(pkg, version, plat string) string {
	return fmt.Sprintf("%s-%s-py3-none-%s.whl", normalize(pkg), version, plat)
}

// recordHash returns the base64url (no-padding) SHA-256 digest of data in the
// "sha256=<digest>" format required by the wheel RECORD spec.
func recordHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256=" + base64.RawURLEncoding.EncodeToString(sum[:])
}

// unixShim uses os.execv to replace the current process — zero subprocess
// overhead.
const unixShim = `import os, sys

def main():
    here = os.path.dirname(os.path.abspath(__file__))
    binary = os.path.join(here, %q)
    os.execv(binary, [binary] + sys.argv[1:])
`

// windowsShim falls back to subprocess because execv is unreliable on Windows.
const windowsShim = `import os, sys, subprocess

def main():
    here = os.path.dirname(os.path.abspath(__file__))
    binary = os.path.join(here, %q)
    sys.exit(subprocess.call([binary] + sys.argv[1:]))
`

// wheelEntry is a single file to be written into the wheel zip.
type wheelEntry struct {
	name string
	data []byte
	exe  bool // set 0o755 instead of 0o644
}

// buildWheel writes a Python wheel containing the given binary, shim package,
// and metadata. It returns the path of the written .whl file.
//
// Parameters:
//   - binaryData: raw bytes of the executable
//   - binaryFilename: filename the binary will have inside the wheel
//   - binVer: upstream binary version string (without leading "v")
//   - cfg: build configuration (package name, repo, etc.)
//   - pyVersion: Python package version string
//   - plat: Python wheel platform tag
//   - descriptionData: Markdown long description
//   - licenseData: license file contents
func buildWheel(
	binaryData []byte,
	binaryFilename, binVer string,
	cfg *Config,
	pyVersion, plat string,
	descriptionData, licenseData []byte,
) (string, error) {
	pkg := cfg.PackageName
	pkgNorm := normalize(pkg)
	isWindows := strings.HasSuffix(binaryFilename, ".exe")

	shimSrc := fmt.Sprintf(unixShim, binaryFilename)
	if isWindows {
		shimSrc = fmt.Sprintf(windowsShim, binaryFilename)
	}

	initSrc := fmt.Sprintf(
		"# %s — generated shim package\n__version__ = %q\n",
		pkg, pyVersion,
	)

	distInfo := fmt.Sprintf("%s-%s.dist-info", pkgNorm, pyVersion)

	licenseExpr := cfg.LicenseExpr
	if licenseExpr == "" {
		licenseExpr = "MIT"
	}

	// Metadata 2.4: long description in message body after blank line.
	metadata := fmt.Sprintf(
		"Metadata-Version: 2.4\n"+
			"Name: %s\n"+
			"Version: %s\n"+
			"Summary: %s\n"+
			"Project-URL: Source, https://github.com/%s\n"+
			"Classifier: Programming Language :: Python :: 3\n"+
			"License-Expression: %s\n"+
			"License-File: LICENSE.txt\n"+
			"Requires-Python: >=3.9\n"+
			"Description-Content-Type: text/markdown; charset=UTF-8; variant=GFM\n"+
			"\n"+
			"%s",
		pkg, pyVersion, cfg.Summary, cfg.Repo, licenseExpr, string(descriptionData),
	)

	wheelMeta := fmt.Sprintf(
		"Wheel-Version: 1.0\nGenerator: buildwheels\nRoot-Is-Purelib: false\nTag: py3-none-%s\n",
		plat,
	)

	entryPoints := fmt.Sprintf(
		"[console_scripts]\n%s = %s._shim:main\n",
		cfg.EntryPoint, pkgNorm,
	)

	entries := []wheelEntry{
		{pkgNorm + "/" + binaryFilename, binaryData, true},
		{pkgNorm + "/__init__.py", []byte(initSrc), false},
		{pkgNorm + "/_shim.py", []byte(shimSrc), false},
		{distInfo + "/METADATA", []byte(metadata), false},
		{distInfo + "/WHEEL", []byte(wheelMeta), false},
		{distInfo + "/entry_points.txt", []byte(entryPoints), false},
		{distInfo + "/licenses/LICENSE.txt", licenseData, false},
	}

	// Build RECORD (path, hash, size per entry; RECORD itself has empty hash/size).
	var rec strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&rec, "%s,%s,%d\n", e.name, recordHash(e.data), len(e.data))
	}
	recordName := distInfo + "/RECORD"
	fmt.Fprintf(&rec, "%s,,\n", recordName)

	// Write the zip.
	// We populate both the 32-bit and 64-bit size fields explicitly to stay
	// in standard zip32 format (avoiding zip64 extra fields that confuse some
	// wheel installers), and set Flags=0 to suppress the data-descriptor bit
	// that causes twine/PyPI to reject the upload.
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	addEntry := func(e wheelEntry) error {
		sz := uint64(len(e.data))
		sz32 := uint32(sz)
		fh := &zip.FileHeader{
			Name:               e.name,
			Method:             zip.Store,
			Flags:              0,
			Modified:           time.Now(),
			CRC32:              crc32.ChecksumIEEE(e.data),
			CompressedSize:     sz32,
			UncompressedSize:   sz32,
			CompressedSize64:   sz,
			UncompressedSize64: sz,
		}
		if e.exe {
			fh.SetMode(0o755)
		} else {
			fh.SetMode(0o644)
		}
		w, err := zw.CreateRaw(fh)
		if err != nil {
			return err
		}
		_, err = w.Write(e.data)
		return err
	}

	for _, e := range entries {
		if err := addEntry(e); err != nil {
			return "", fmt.Errorf("adding %s: %w", e.name, err)
		}
	}
	// RECORD must be the last entry.
	if err := addEntry(wheelEntry{recordName, []byte(rec.String()), false}); err != nil {
		return "", fmt.Errorf("adding RECORD: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("closing zip: %w", err)
	}

	out := filepath.Join(cfg.Output, wheelFilename(pkg, pyVersion, plat))
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("write wheel: %w", err)
	}
	slog.Debug("wrote wheel", "path", out)
	return out, nil
}
