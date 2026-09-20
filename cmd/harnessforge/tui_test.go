package main

import (
	"bytes"
	"harnessforge/internal/harness"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTUICreatesHarnessAfterConfirmation(t *testing.T) {
	dir := t.TempDir()
	var output bytes.Buffer
	if err := runTUI(strings.NewReader("1\ny\n5\n"), &output, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".harness", "harness.yaml")); err != nil {
		t.Fatalf("harness was not created: %v", err)
	}
	if !strings.Contains(output.String(), "Created .harness/harness.yaml") {
		t.Fatalf("unexpected output: %s", output.String())
	}
}

func TestInstallCommandExposesTUIAsAlias(t *testing.T) {
	command := newTUICommand()
	if command.Use != "install [repository]" {
		t.Fatalf("command use = %q", command.Use)
	}
	if len(command.Aliases) != 1 || command.Aliases[0] != "tui" {
		t.Fatalf("aliases = %v, want tui", command.Aliases)
	}
}

func TestTUICancelsCreateWithoutChangingRepository(t *testing.T) {
	dir := t.TempDir()
	var output bytes.Buffer
	if err := runTUI(strings.NewReader("1\nn\n5\n"), &output, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".harness", "harness.yaml")); !os.IsNotExist(err) {
		t.Fatalf("harness created after cancellation: %v", err)
	}
	if !strings.Contains(output.String(), "Cancelled; no changes made.") {
		t.Fatalf("unexpected output: %s", output.String())
	}
}

func TestTUIKeepsExistingHarness(t *testing.T) {
	dir := t.TempDir()
	if _, err := harness.Init(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".harness", "harness.yaml")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runTUI(strings.NewReader("1\n5\n"), &output, dir); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatalf("existing harness changed: %v", err)
	}
	if !strings.Contains(output.String(), "already exists; left it unchanged") {
		t.Fatalf("missing guidance: %s", output.String())
	}
}

func TestTUIGeneratesOnlyAfterConfirmation(t *testing.T) {
	dir := t.TempDir()
	if _, err := harness.Init(dir); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runTUI(strings.NewReader("3\ny\n5\n"), &output, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatalf("AGENTS.md was not generated: %v", err)
	}
}
