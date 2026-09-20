package harness

import (
	"fmt"
	"harnessforge/internal/securityboundary"
	"os"
	"path/filepath"
	"strconv"
)

const defaultHarnessYAML = `version: 1

project:
  name: %s
`

func Init(repositoryPath string) (string, error) {
	harnessDir, err := securityboundary.PrepareDirectory(repositoryPath, ".harness")
	if err != nil {
		return "", err
	}
	harnessFile := filepath.Join(harnessDir, "harness.yaml")
	root, err := filepath.Abs(repositoryPath)
	if err != nil {
		return "", err
	}

	file, err := os.OpenFile(harnessFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := file.WriteString(fmt.Sprintf(defaultHarnessYAML, strconv.Quote(filepath.Base(root)))); err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Join(".harness", "harness.yaml")), nil
}
