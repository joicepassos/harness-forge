package analyzer

import (
	"os"
	"path/filepath"
	"strings"
)

func fileContains(repositoryPath string, path string, text string) bool {
	content, err := os.ReadFile(filepath.Join(repositoryPath, path))
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(string(content)), strings.ToLower(text))
}
