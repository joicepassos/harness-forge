package infrastructure

import (
	"bytes"
	"strings"
	"testing"

	"harnessforge/internal/generation/domain"
)

func TestCodexAdapterRendersScopedDeterministicAGENTS(t *testing.T) {
	input := domain.Input{
		Project: "fixture-monorepo",
		Rules: []domain.Rule{
			{ID: "web-style", Description: "Use the shared UI components.", Paths: []string{"apps/web/"}},
			{ID: "repo-basics", Description: "Keep changes focused."},
		},
	}

	first, err := (CodexAdapter{}).Render(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := (CodexAdapter{}).Render(input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != "AGENTS.md" {
		t.Fatalf("Codex output path = %q, want AGENTS.md", first.Path)
	}
	if !bytes.Equal(first.Content, second.Content) {
		t.Fatal("same fixture input produced different Codex output")
	}
	for _, expected := range []string{
		"# fixture-monorepo agent instructions",
		"## Scope and precedence",
		"## Approved rules",
		"[repo-basics] Keep changes focused.",
		"[repo-basics] Keep changes focused. (global)",
		"[web-style] Use the shared UI components. (advisory; applies only to paths matching: `apps/web/`)",
	} {
		if !bytes.Contains(first.Content, []byte(expected)) {
			t.Errorf("Codex AGENTS.md missing %q:\n%s", expected, first.Content)
		}
	}
	if bytes.Index(first.Content, []byte("[repo-basics]")) > bytes.Index(first.Content, []byte("[web-style]")) {
		t.Fatal("rules are not rendered in deterministic ID order")
	}
}

func TestCodexAdapterKeepsStaticScopeAdvisoryWithoutImplicitPrecedence(t *testing.T) {
	input := domain.Input{
		Project: "fixture-monorepo",
		Rules: []domain.Rule{
			{ID: "z-global", Description: "Keep changes focused."},
			{ID: "broad", Description: "Use service conventions.", Paths: []string{"services/**"}},
			{ID: "narrow", Description: "Use API conventions.", Paths: []string{"services/api/**"}},
		},
	}
	doc, err := (CodexAdapter{}).Render(input)
	if err != nil {
		t.Fatal(err)
	}
	content := string(doc.Content)
	rulesStart := strings.Index(content, "## Approved rules")
	if rulesStart < 0 {
		t.Fatalf("approved rules section missing:\n%s", content)
	}
	rules := content[rulesStart:]
	for _, exact := range []string{
		"[broad] Use service conventions. (advisory; applies only to paths matching: `services/**`)",
		"[narrow] Use API conventions. (advisory; applies only to paths matching: `services/api/**`)",
		"[z-global] Keep changes focused. (global)",
	} {
		if !strings.Contains(rules, exact) {
			t.Errorf("rule lost its exact scope annotation %q:\n%s", exact, rules)
		}
	}
	for _, contract := range []string{
		"Rules without path scopes apply globally.",
		"Path scopes retain their exact authored globs and are advisory in this static export; the agent does not enforce glob matching.",
		"Matching global and scoped rules coexist with no implicit precedence; conflicting rules require explicit reconciliation.",
	} {
		if !strings.Contains(content, contract) {
			t.Errorf("static scope contract missing %q:\n%s", contract, content)
		}
	}
	if strings.Index(rules, "[broad]") > strings.Index(rules, "[narrow]") || strings.Index(rules, "[narrow]") > strings.Index(rules, "[z-global]") {
		t.Errorf("rules are not sorted by ID:\n%s", rules)
	}
}
