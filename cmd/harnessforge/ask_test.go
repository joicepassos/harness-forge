package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAskContextExplainPrintsPlanWithoutProvider(t *testing.T) {
	root := t.TempDir()
	full := filepath.Join(root, "internal", "webhook", "handler.go")
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("webhook authentication persistence monitoring"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newAskCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--format", "json", "--repository", root, "--context-explain", "How are webhooks authenticated?"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var plan struct {
		BudgetTokens int `json:"budget_tokens"`
		Included     []struct {
			Source string `json:"source"`
			Text   string `json:"text"`
		} `json:"included"`
		Excluded []struct {
			Reason string `json:"reason"`
		} `json:"excluded"`
	}
	if err := json.Unmarshal(output.Bytes(), &plan); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, output.String())
	}
	if plan.BudgetTokens == 0 || len(plan.Included) == 0 {
		t.Fatalf("missing context plan: %s", output.String())
	}
	if !strings.Contains(plan.Included[0].Text, "webhook") {
		t.Fatalf("unexpected included context: %s", output.String())
	}
}
