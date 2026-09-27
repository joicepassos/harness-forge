package infrastructure

import (
	"bytes"
	"harnessforge/internal/generation/domain"
	"strings"
	"testing"
)

func TestClaudeAdapterRendersProjectInstructionsDeterministically(t *testing.T) {
	input := domain.Input{
		Project: "sample",
		Rules: []domain.Rule{
			{ID: "z-rule", Description: "Last rule"},
			{ID: "a-rule", Description: "Scoped rule", Paths: []string{"services/api/**", "libs/core/**"}},
		},
		Skills: []domain.Skill{{ID: "api", Description: "API conventions", Path: ".harness/skills/api/SKILL.md"}},
		Gates: []domain.QualityGate{{ID: "api-tests", Command: "go test ./...", Workspace: "services/api", Workspaces: []string{"services/api", "libs/core"}}},
	}
	adapter := ClaudeAdapter{}
	first, err := adapter.Render(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Render(input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != "CLAUDE.md" {
		t.Fatalf("path = %q, want CLAUDE.md", first.Path)
	}
	if !bytes.Equal(first.Content, second.Content) {
		t.Fatal("Claude output is not deterministic")
	}
	for _, want := range []string{
		"# sample agent instructions", "[a-rule] Scoped rule", "services/api/**", "libs/core/**",
		".harness/skills/api/SKILL.md", "api-tests", "go test ./...", "workspace: `services/api`",
		"workspaces: `services/api`, `libs/core`",
	} {
		if !strings.Contains(string(first.Content), want) {
			t.Errorf("CLAUDE.md omits %q:\n%s", want, first.Content)
		}
	}
	if strings.Index(string(first.Content), "[a-rule]") > strings.Index(string(first.Content), "[z-rule]") {
		t.Fatal("rules are not emitted in stable ID order")
	}
}
