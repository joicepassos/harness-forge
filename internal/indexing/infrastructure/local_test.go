package infrastructure

import (
	"context"
	"harnessforge/internal/indexing/domain"
	"os"
	"path/filepath"
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
