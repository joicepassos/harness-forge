package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These clones exercise the files a downstream agent receives after export.
// They intentionally contain no Forge config or generated ownership state.
func TestWithoutForgeClonesPreserveExportedContract(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "clones", "without-forge")
	for _, agent := range []struct {
		name, instruction string
	}{
		{name: "codex", instruction: "AGENTS.md"},
		{name: "claude", instruction: "CLAUDE.md"},
	} {
		t.Run(agent.name, func(t *testing.T) {
			clone := filepath.Join(root, agent.name)
			for _, forbidden := range []string{
				filepath.Join(clone, ".forge"), filepath.Join(clone, ".harness"),
				filepath.Join(clone, ".forge", "generated-manifest.json"),
			} {
				if _, err := os.Lstat(forbidden); !os.IsNotExist(err) {
					t.Fatalf("consumer clone contains Forge state %s: %v", forbidden, err)
				}
			}
			instruction, err := os.ReadFile(filepath.Join(clone, agent.instruction))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{
				"PAY-001", "idempotent", "PAY-002", "typed error", "PAY-003",
				"PaymentProvider", "SKILL-001", "docs/payment-provider-change.md",
				"GATE-001", "go test ./...",
			} {
				if !strings.Contains(string(instruction), required) {
					t.Errorf("%s export is missing %q", agent.instruction, required)
				}
			}
			skill, err := os.ReadFile(filepath.Join(clone, "docs", "payment-provider-change.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(skill), "idempotency") || !strings.Contains(string(skill), "amount and currency") {
				t.Errorf("%s clone skill is missing its payment procedure", agent.name)
			}
			if _, err := os.Stat(filepath.Join(clone, "go.mod")); err != nil {
				t.Errorf("%s clone lacks its independent project: %v", agent.name, err)
			}
		})
	}
}
