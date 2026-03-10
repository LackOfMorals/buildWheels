// archive.go — extraction of binaries from .tar.gz and .zip archives.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
)

// extractFromTarGz finds the entry whose basename matches target inside a
// .tar.gz archive and returns its raw bytes.
func extractFromTarGz(data []byte, target string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if path.Base(hdr.Name) == target {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%q not found in tar.gz archive", target)
}

// extractFromZip finds the entry whose basename matches target inside a
// zip archive and returns its raw bytes.
func extractFromZip(data []byte, target string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
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

// extractBinary delegates to the archive-specific extractor based on ext
// ("tar.gz" or "zip").
func extractBinary(archiveData []byte, ext, binaryFilename string) ([]byte, error) {
	switch ext {
	case "tar.gz":
		return extractFromTarGz(archiveData, binaryFilename)
	case "zip":
		return extractFromZip(archiveData, binaryFilename)
	default:
		return nil, fmt.Errorf("unsupported archive type: %q", ext)
	}
}
