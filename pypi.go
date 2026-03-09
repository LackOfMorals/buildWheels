// pypi.go — upload of a built wheel to a PyPI-compatible index.
package main

import (
	"bytes"
	"crypto/md5" //nolint:gosec // MD5 required by PyPI legacy upload API
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

// wheelDigests returns the MD5 and SHA-256 hex digests of the wheel bytes.
// Both are required by the PyPI legacy upload API.
func wheelDigests(data []byte) (md5hex, sha256hex string) {
	m := md5.Sum(data) //nolint:gosec
	s := sha256.Sum256(data)
	return fmt.Sprintf("%x", m), fmt.Sprintf("%x", s)
}

// uploadToPyPI uploads a single wheel file to a PyPI-compatible legacy upload
// endpoint. username is "__token__" when using an API token as the password.
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
		"pyversion":        "py30",
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
			slog.Warn("wheel already exists on PyPI, skipping", "file", filename)
			return nil
		}
		return fmt.Errorf("PyPI 400: %s", msg)
	default:
		return fmt.Errorf("PyPI %s: %s", resp.Status, string(respBody))
	}
}
