package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesHarnessFile(t *testing.T) {
	dir := t.TempDir()

	path, err := Init(dir)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if path != ".harness/harness.yaml" {
		t.Fatalf("Init() path = %q, want %q", path, ".harness/harness.yaml")
	}

	content, err := os.ReadFile(filepath.Join(dir, ".harness", "harness.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	want := `version: 1

project:
  name: my-project
`
	if string(content) != want {
		t.Fatalf("harness.yaml = %q, want %q", string(content), want)
	}
}
