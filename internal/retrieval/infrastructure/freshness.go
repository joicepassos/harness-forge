package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
	indexdomain "harnessforge/internal/indexing/domain"
	"os"
	"path/filepath"
	"sort"
)

type Files struct{}

func (Files) Stale(root string, index indexdomain.Index) ([]string, error) {
	expected := map[string]string{}
	for _, chunk := range index.Chunks {
		expected[chunk.Source] = chunk.SourceHash
	}
	var stale []string
	for source, want := range expected {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source)))
		if err != nil || hash(data) != want {
			stale = append(stale, source)
		}
	}
	sort.Strings(stale)
	return stale, nil
}
func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
