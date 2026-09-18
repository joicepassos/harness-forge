package infrastructure

import (
	"context"
	"harnessforge/internal/indexing/domain"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDocumentsAndStructuralChunksRespectBoundaries(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0700)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# First\nAlpha\n# Second\nBeta"), 0600)
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package fixture"), 0600)
	os.WriteFile(filepath.Join(dir, ".git", "hidden.md"), []byte("secret"), 0600)
	documents, err := (Documents{}).Documents(context.Background(), dir)
	if err != nil || len(documents) != 1 {
		t.Fatalf("%#v %v", documents, err)
	}
	chunks := (Markdown{}).Chunks(documents[0])
	if len(chunks) != 2 || chunks[0].StartLine != 1 || chunks[1].StartLine != 3 {
		t.Fatalf("%#v", chunks)
	}
}
func TestDocumentsExcludeIgnoredAndSensitiveContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("private/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "private"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "private", "notes.md"), []byte("not for indexing"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("api_key=fictional-credential-value"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "safe.md"), []byte("safe documentation"), 0600); err != nil {
		t.Fatal(err)
	}
	documents, err := (Documents{}).Documents(context.Background(), dir)
	if err != nil || len(documents) != 1 || documents[0].Path != "safe.md" {
		t.Fatalf("documents=%#v err=%v", documents, err)
	}
}
func TestJSONStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	index := domain.Index{Version: "index-v1", Model: "m", Dimensions: 2, Chunks: []domain.Chunk{{ID: "one"}}}
	store := JSONStore{}
	if err := store.Save(dir, index); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(dir)
	if err != nil || loaded.Version != "index-v1" || len(loaded.Chunks) != 1 {
		t.Fatal(loaded, err)
	}
}
func TestJSONStoreRejectsRedirectedHarnessDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires developer mode or elevated privileges")
	}
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, ".harness")); err != nil {
		t.Fatal(err)
	}
	if err := (JSONStore{}).Save(dir, domain.Index{}); err == nil {
		t.Fatal("redirected .harness directory accepted")
	}
}
