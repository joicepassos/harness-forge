package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLLoaderRejectsOversizedHarness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harness.yaml")
	if err := os.WriteFile(path, make([]byte, 4<<20+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (YAMLLoader{}).Load(path); err == nil {
		t.Fatal("oversized Harness YAML accepted")
	}
}
