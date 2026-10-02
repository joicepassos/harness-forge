package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewExplicitMissingForgeDoesNotFallBackToHarness(t *testing.T) {
	root := t.TempDir()
	harnessPath := filepath.Join(root, ".harness", "harness.yaml")
	if err := os.MkdirAll(filepath.Dir(harnessPath), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte("version: 1\nproject: {name: sample}\nrules:\n  - id: sample-rule\n    description: Example\n    origin: human\n    status: candidate\n")
	if err := os.WriteFile(harnessPath, original, 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newReviewCommand()
	cmd.SetArgs([]string{"sample-rule", "approved", "--layout", "forge", "--repository", root, "--file", harnessPath})
	cmd.SetOut(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "selected forge layout is missing") {
		t.Fatalf("review error = %v; want missing explicit Forge layout", err)
	}
	after, err := os.ReadFile(harnessPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatal("explicit missing Forge selection modified the legacy Harness file")
	}
}
