package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	indexdomain "harnessforge/internal/indexing/domain"
	"harnessforge/internal/inputlimits"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Files struct{}

func (Files) Stale(root string, index indexdomain.Index) ([]string, error) {
	expected := map[string]string{}
	for _, chunk := range index.Chunks {
		expected[chunk.Source] = chunk.SourceHash
	}
	var stale []string
	for source, want := range expected {
		data, err := containedRegularFile(root, source)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			stale = append(stale, source)
			continue
		}
		if hash(data) != want {
			stale = append(stale, source)
		}
	}
	sort.Strings(stale)
	return stale, nil
}
func containedRegularFile(root, source string) ([]byte, error) {
	if source == "" || strings.Contains(source, "\\") || filepath.IsAbs(source) || source == ".." || strings.HasPrefix(source, "../") || filepath.Clean(source) != filepath.FromSlash(source) {
		return nil, fmt.Errorf("index source is not repository-relative")
	}
	full := filepath.Join(root, filepath.FromSlash(source))
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("index source escapes repository")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("index source is not a regular file")
	}
	return inputlimits.ReadFile(resolved, inputlimits.SourceFileBytes, "index source")
}
func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
