package utiles

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeTestTarGz(t *testing.T, path string, entries []tar.Header) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, h := range entries {
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatalf("failed to write header: %v", err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte("x")); err != nil {
				t.Fatalf("failed to write content: %v", err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip: %v", err)
	}
}

func TestDecompressTarGzRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "evil.tar.gz")
	writeTestTarGz(t, archive, []tar.Header{
		{Name: "../escape.txt", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg},
	})

	dest := filepath.Join(dir, "out")
	err := decompressTarGz(archive, dest)
	if err == nil {
		t.Fatal("expected traversal to be rejected")
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.txt")); !os.IsNotExist(err) {
		t.Fatal("traversal file was written outside dest")
	}
}

func TestDecompressTarGzRejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "abs.tar.gz")
	writeTestTarGz(t, archive, []tar.Header{
		{Name: "/etc/evil", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg},
	})

	err := decompressTarGz(archive, filepath.Join(dir, "out"))
	if err == nil {
		t.Fatal("expected absolute path entry to be rejected")
	}
}

func TestDecompressTarGzRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "link.tar.gz")
	writeTestTarGz(t, archive, []tar.Header{
		{Name: "link", Mode: 0o777, Size: 0, Typeflag: tar.TypeSymlink, Linkname: "/etc"},
	})

	err := decompressTarGz(archive, filepath.Join(dir, "out"))
	if err == nil {
		t.Fatal("expected symlink entry to be rejected")
	}
}

func TestDecompressTarGzExtractsNormalFile(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "ok.tar.gz")
	writeTestTarGz(t, archive, []tar.Header{
		{Name: "dist/linux/amd64/dockerCopilot", Mode: 0o755, Size: 1, Typeflag: tar.TypeReg},
	})

	dest := filepath.Join(dir, "out")
	if err := decompressTarGz(archive, dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	extracted := filepath.Join(dest, "dist/linux/amd64/dockerCopilot")
	info, err := os.Stat(extracted)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected mode 0755, got %v", info.Mode().Perm())
	}
}
