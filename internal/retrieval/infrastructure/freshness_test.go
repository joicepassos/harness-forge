package infrastructure

import (
	indexdomain "harnessforge/internal/indexing/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestFreshnessDetectsChangedAndDeletedSources(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "same.md"), []byte("same"), 0600)
	os.WriteFile(filepath.Join(dir, "changed.md"), []byte("new"), 0600)
	index := indexdomain.Index{Chunks: []indexdomain.Chunk{{Source: "same.md", SourceHash: hash([]byte("same"))}, {Source: "changed.md", SourceHash: hash([]byte("old"))}, {Source: "deleted.md", SourceHash: hash([]byte("old"))}}}
	stale, err := (Files{}).Stale(dir, index)
	if err != nil || len(stale) != 2 || stale[0] != "changed.md" || stale[1] != "deleted.md" {
		t.Fatal(stale, err)
	}
}
