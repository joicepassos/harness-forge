package infrastructure

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileReaderFindsSymbolsAndRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("func Present() {}"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := NewFileReader(root)
	if err != nil {
		t.Fatal(err)
	}
	found, err := reader.Contains(context.Background(), "source.go", "Present")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if _, err := reader.Contains(context.Background(), "../outside", "Present"); err == nil {
		t.Fatal("traversal accepted")
	}
}

func TestFileReaderRejectsExternalSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.go"), []byte("Present"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.go")
	if err := os.Symlink(filepath.Join(outside, "outside.go"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	reader, err := NewFileReader(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Contains(context.Background(), "link.go", "Present"); err == nil {
		t.Fatal("external symlink accepted")
	}
}
