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

	file, err := os.OpenFile(harnessFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := file.WriteString(defaultHarnessYAML); err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Join(".harness", "harness.yaml")), nil
}
