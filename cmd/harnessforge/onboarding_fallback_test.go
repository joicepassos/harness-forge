package main

import (
	"bytes"
	"context"
	"errors"
	"harnessforge/internal/analyzer"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuidedInitProviderFailureCanCompleteLocally(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	var output bytes.Buffer
	answers := []string{"1", "", "", "y", "deepseek", "", "", "", "y", "y", "3", "y", ""}
	called := false
	propose := func(context.Context, setupProvider, *analyzer.Analysis, []setupDocument, string) (setupAIProposal, error) {
		called = true
		return setupAIProposal{}, errors.New("deepseek request failed (HTTP 400): code invalid_request: unsupported parameter")
	}
	if err := runGuidedInit(context.Background(), strings.NewReader(strings.Join(answers, "\n")), &output, root, propose); err != nil {
		t.Fatal(err)
	}
	if !called || !strings.Contains(output.String(), "unsupported parameter") || !strings.Contains(output.String(), "Setup complete") {
		t.Fatalf("fallback missing: %s", output.String())
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", ".harness/harness.yaml"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, ".harness/harness.yaml"))
	if err != nil || bytes.Contains(content, []byte("deepseek")) || bytes.Contains(content, []byte("test-key")) {
		t.Fatalf("fallback retained AI credentials/config: %v", err)
	}
}
