package safefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	temporary := filepath.Join(dir, "temporary")
	destination := filepath.Join(dir, "destination")
	if err := os.WriteFile(temporary, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Replace(temporary, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "new" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}
