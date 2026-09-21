package main

import (
	"bytes"
	"encoding/json"
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
