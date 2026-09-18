package analyzer

import (
	"harnessforge/internal/inputlimits"
	"path/filepath"
	"strings"
)

func (repository Repository) FileContains(path string, text string) bool {
	for _, file := range repository.Files {
		if !samePathOrBase(file, path) {
			continue
		}

		content, err := inputlimits.ReadFile(filepath.Join(repository.Path, file), inputlimits.SourceFileBytes, "source file")
		if err != nil {
			continue
		}

		if strings.Contains(strings.ToLower(string(content)), strings.ToLower(text)) {
			return true
		}
	}

	return false
}
