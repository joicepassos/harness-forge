package infrastructure

import (
	"bytes"
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
		"[repo-basics] Keep changes focused.",
		"[web-style] Use the shared UI components. (scope: [apps/web/])",
	} {
		if !bytes.Contains(first.Content, []byte(expected)) {
			t.Errorf("Codex AGENTS.md missing %q:\n%s", expected, first.Content)
		}
	}
	if bytes.Index(first.Content, []byte("[repo-basics]")) > bytes.Index(first.Content, []byte("[web-style]")) {
		t.Fatal("rules are not rendered in deterministic ID order")
	}
}
