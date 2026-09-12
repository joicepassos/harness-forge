package main

import (
	"bytes"
	"harnessforge/internal/harness"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateInitAndPreserveManualFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := harness.Init(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".harness", "harness.yaml")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original = append([]byte("# Manual comment\n"), original...)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newValidateCommand()
	cmd.SetArgs([]string{path})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := harness.Init(dir); err == nil {
		t.Fatal("init overwrote manual file")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) {
		t.Fatal("manual content changed")
	}
}
func TestValidateRejectsInvalidDocuments(t *testing.T) {
	base := "version: 1\nproject:\n  name: sample-project\n"
	rule := "rules:\n  - id: sample\n    description: Example\n    origin: human\n    status: candidate\n"
	cases := []struct{ name, text, want string }{
		{"null collection", base + "rules: null\n", "null is not allowed"},
		{"version", strings.Replace(base, "version: 1", "version: 2", 1), "version"},
		{"missing name", "version: 1\nproject: {}", "project.name"},
		{"unknown field", base + "secret: value\n", "unknown field"},
		{"wrong type", strings.Replace(base, "name: sample-project", "name: 123", 1), "cannot unmarshal"},
		{"duplicate key", base + "version: 1\n", "already defined"},
		{"multiple documents", base + "---\n" + base, "one YAML document"},
		{"duplicate id", base + rule + "  - id: sample\n    description: Other\n    origin: human\n    status: approved\n", "duplicate"},
		{"missing evidence", base + strings.Replace(rule, "origin: human", "origin: ai", 1), "evidence"},
		{"invalid status", base + strings.Replace(rule, "candidate", "automatic", 1), "status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "harness.yaml")
			if err := os.WriteFile(path, []byte(tc.text), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := newValidateCommand()
			cmd.SetArgs([]string{path})
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want %s", err, tc.want)
			}
		})
	}
}
