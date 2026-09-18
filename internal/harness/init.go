package harness

import (
	"harnessforge/internal/securityboundary"
	"os"
	"path/filepath"
)

const defaultHarnessYAML = `version: 1

project:
  name: my-project
`

func Init(repositoryPath string) (string, error) {
	harnessDir, err := securityboundary.PrepareDirectory(repositoryPath, ".harness")
	if err != nil {
		return "", err
	}
	harnessFile := filepath.Join(harnessDir, "harness.yaml")

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
