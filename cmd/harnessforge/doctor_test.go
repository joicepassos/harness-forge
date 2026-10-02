package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"harnessforge/internal/harness"
)

func TestDoctorFixKeepsJSONOnStdout(t *testing.T) {
	dir := t.TempDir()
	if _, err := harness.Init(dir); err != nil {
		t.Fatal(err)
	}
	cmd := newDoctorCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--fix", filepath.Join(dir, ".harness", "harness.yaml")})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v: %q", err, stdout.String())
	}
	if !strings.Contains(stderr.String(), "Proposals only") || strings.Contains(stdout.String(), "Proposals only") {
		t.Fatalf("unexpected outputs: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDoctorDiscoversForgeLayoutFromRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".forge"), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := "layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences: {}\n"
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newDoctorCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--repository", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v: %q", err, stdout.String())
	}
}
