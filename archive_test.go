// archive_test.go
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"
)

// makeTarGz builds an in-memory .tar.gz from a map of filename→content.
func makeTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar WriteHeader: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("tar Write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

// makeZip builds an in-memory zip from a map of filename→content.
func makeZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip Create: %v", err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("zip Write: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractFromTarGz_Found(t *testing.T) {
	want := []byte("hello binary")
	data := makeTarGz(t, map[string][]byte{
		"subdir/mybinary": want,
		"other.txt":       []byte("noise"),
	})
	got, err := extractFromTarGz(data, "mybinary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractFromTarGz_NotFound(t *testing.T) {
	data := makeTarGz(t, map[string][]byte{"other.txt": []byte("noise")})
	_, err := extractFromTarGz(data, "missing")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestExtractFromTarGz_NestedPath(t *testing.T) {
	// Extraction should match on basename, ignoring directory prefix.
	want := []byte("deep")
	data := makeTarGz(t, map[string][]byte{"a/b/c/tool": want})
	got, err := extractFromTarGz(data, "tool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("content mismatch")
	}
}

func TestExtractFromZip_Found(t *testing.T) {
	want := []byte("zip binary content")
	data := makeZip(t, map[string][]byte{
		"subdir/tool.exe": want,
		"README.md":       []byte("docs"),
	})
	got, err := extractFromZip(data, "tool.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractFromZip_NotFound(t *testing.T) {
	data := makeZip(t, map[string][]byte{"readme.md": []byte("docs")})
	_, err := extractFromZip(data, "missing.exe")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestExtractBinary_TarGz(t *testing.T) {
	want := []byte("binary data")
	data := makeTarGz(t, map[string][]byte{"mytool": want})
	got, err := extractBinary(data, "tar.gz", "mytool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("content mismatch")
	}
}

func TestExtractBinary_Zip(t *testing.T) {
	want := []byte("exe data")
	data := makeZip(t, map[string][]byte{"mytool.exe": want})
	got, err := extractBinary(data, "zip", "mytool.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("content mismatch")
	}
}

func TestExtractBinary_UnknownExt(t *testing.T) {
	_, err := extractBinary([]byte("data"), "rar", "file")
	if err == nil {
		t.Fatal("expected error for unknown archive type, got nil")
	}
}

func TestExtractBinary_CorruptData(t *testing.T) {
	_, err := extractBinary([]byte("not a valid archive"), "tar.gz", "file")
	if err == nil {
		t.Fatal("expected error for corrupt archive, got nil")
	}
}
