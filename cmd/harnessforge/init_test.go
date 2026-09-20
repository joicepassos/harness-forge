package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCommandKeepsExistingHarness(t *testing.T) {
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	var output bytes.Buffer
	first := newRootCommand()
	first.SetArgs([]string{"init"})
	first.SetOut(&output)
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".harness", "harness.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	output.Reset()
	second := newRootCommand()
	second.SetArgs([]string{"init"})
	second.SetOut(&output)
	if err := second.Execute(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(content, after) {
		t.Fatalf("existing harness changed: %v", err)
	}
	if !strings.Contains(output.String(), "already exists; left it unchanged") {
		t.Fatalf("missing next step: %s", output.String())
	}
}
