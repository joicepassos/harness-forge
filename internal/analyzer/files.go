package analyzer

import (
	"os"
	"path/filepath"
	"strings"
)

func (repository Repository) FileContains(path string, text string) bool {
	content, err := os.ReadFile(filepath.Join(repository.Path, path))
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(string(content)), strings.ToLower(text))
}
