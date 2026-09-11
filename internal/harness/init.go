package harness

import (
	"os"
	"path/filepath"
)

const defaultHarnessYAML = `version: 1

project:
  name: my-project
`

func Init(repositoryPath string) (string, error) {
	harnessDir := filepath.Join(repositoryPath, ".harness")
	harnessFile := filepath.Join(harnessDir, "harness.yaml")

	if err := os.MkdirAll(harnessDir, 0755); err != nil {
		return "", err
	}

	if err := os.WriteFile(harnessFile, []byte(defaultHarnessYAML), 0644); err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Join(".harness", "harness.yaml")), nil
}
