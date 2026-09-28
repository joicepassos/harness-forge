package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncCommandTargetsExplicitRepository(t *testing.T) {
	target := t.TempDir()
	manifest := filepath.Join(target, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("layout_version: 1\nir_version: 2\nproject: {name: target}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCommand()
	cmd.SetArgs([]string{"sync", "--apply", "--repository", target})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "AGENTS.md")); err != nil {
		t.Fatalf("sync did not publish into selected repository: %v", err)
	}
}
