package infrastructure

import (
	"bytes"
	"harnessforge/internal/discovery/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreAppendsOnceAndPreservesManualBytes(t *testing.T) {
	dir := t.TempDir()
	harness := filepath.Join(dir, "harness.yaml")
	source := filepath.Join(dir, "port.go")
	original := []byte("# manual comment\nversion: 1\nproject: {name: sample}\n")
	if err := os.WriteFile(harness, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("type Port interface{}"), 0600); err != nil {
		t.Fatal(err)
	}
	proposal := domain.Proposal{ID: "use-ports", Description: "Use ports", Evidence: []domain.Evidence{{File: "port.go", Symbol: "Port"}}}
	if err := (Store{}).Apply(dir, harness, proposal); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(harness)
	if !bytes.HasPrefix(first, original) || !bytes.Contains(first, []byte("status: approved")) {
		t.Fatalf("%s", first)
	}
	if err := (Store{}).Apply(dir, harness, proposal); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(harness)
	if !bytes.Equal(first, second) {
		t.Fatal("duplicate application changed harness")
	}
}
func TestStoreRejectsInvalidEvidenceWithoutChangingHarness(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "harness.yaml")
	original := []byte("version: 1\nproject: {name: sample}\n")
	os.WriteFile(path, original, 0600)
	proposal := domain.Proposal{ID: "unsafe", Description: "Unsafe", Evidence: []domain.Evidence{{File: "../outside"}}}
	if err := (Store{}).Apply(dir, path, proposal); err == nil {
		t.Fatal("unsafe evidence accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(original, after) {
		t.Fatal("failed application changed harness")
	}
}
