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

func TestCodexAdapterKeepsFileGlobsStaticAndAdvisory(t *testing.T) {
	input := domain.Input{
		Project: "fixture-monorepo",
		Rules: []domain.Rule{
			{ID: "z-global", Description: "Keep changes focused."},
			{ID: "broad", Description: "Use service conventions.", Paths: []string{"services/**/*.go"}},
			{ID: "narrow", Description: "Use API conventions.", Paths: []string{"services/api/*.go"}},
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
		"[broad] Use service conventions. (advisory; applies only to paths matching: `services/**/*.go`)",
		"[narrow] Use API conventions. (advisory; applies only to paths matching: `services/api/*.go`)",
		"[z-global] Keep changes focused. (global)",
	} {
		if !strings.Contains(rules, exact) {
			t.Errorf("rule lost its exact scope annotation %q:\n%s", exact, rules)
		}
	}
	for _, contract := range []string{
		"Rules without path scopes apply globally.",
		"Other path globs retain their exact authored text and are advisory; Codex does not enforce file-glob matching from this export.",
		"Global and directory-scoped rules coexist; conflicting rules require explicit reconciliation.",
	} {
		if !strings.Contains(content, contract) {
			t.Errorf("static scope contract missing %q:\n%s", contract, content)
		}
	}
	if strings.Index(rules, "[broad]") > strings.Index(rules, "[narrow]") || strings.Index(rules, "[narrow]") > strings.Index(rules, "[z-global]") {
		t.Errorf("rules are not sorted by ID:\n%s", rules)
	}
}

func TestCodexAdapterEmitsNativeDirectoryScopesAsDeterministicNestedInstructions(t *testing.T) {
	input := domain.Input{
		Project: "fixture-monorepo",
		Rules: []domain.Rule{
			{ID: "global", Description: "Use repository conventions."},
			{ID: "repo-scope", Description: "Apply throughout repository.", Paths: []string{"**"}},
			{ID: "service", Description: "Use service conventions.", Paths: []string{"services/**"}},
			{ID: "api", Description: "Use API conventions.", Paths: []string{"services/api/**"}},
			{ID: "mixed", Description: "Keep Go APIs stable.", Paths: []string{"services/api/**", "services/api/**/*.go"}},
		},
	}
	first, err := (CodexAdapter{}).RenderDocuments(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := (CodexAdapter{}).RenderDocuments(input)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{"AGENTS.md", "services/AGENTS.md", "services/api/AGENTS.md"}
	if len(first) != len(wantPaths) {
		t.Fatalf("got documents %#v, want paths %v", first, wantPaths)
	}
	for i, wantPath := range wantPaths {
		if first[i].Path != wantPath || second[i].Path != wantPath {
			t.Fatalf("documents[%d].Path = %q/%q, want %q", i, first[i].Path, second[i].Path, wantPath)
		}
		if !bytes.Equal(first[i].Content, second[i].Content) {
			t.Fatalf("same input produced different bytes for %s", wantPath)
		}
	}
	root := string(first[0].Content)
	if !strings.Contains(root, "[global] Use repository conventions. (global)") ||
		!strings.Contains(root, "[repo-scope] Apply throughout repository. (native Codex directory scope: `**`)") ||
		!strings.Contains(root, "[mixed] Keep Go APIs stable. (advisory; applies only to paths matching: `services/api/**`, `services/api/**/*.go`)") {
		t.Fatalf("root instructions lost a global, repo-wide, or non-representable scope:\n%s", root)
	}
	if strings.Contains(root, "[service]") || strings.Contains(root, "[api]") {
		t.Fatalf("nested rules leaked into root instructions:\n%s", root)
	}
	service := string(first[1].Content)
	if !strings.Contains(service, "[service] Use service conventions. (native Codex directory scope: `services/**`)") || strings.Contains(service, "[api]") {
		t.Fatalf("services instructions do not contain only their local scope:\n%s", service)
	}
	api := string(first[2].Content)
	if !strings.Contains(api, "[api] Use API conventions. (native Codex directory scope: `services/api/**`)") || strings.Contains(api, "[service]") {
		t.Fatalf("API instructions do not contain only their local scope:\n%s", api)
	}
	if _, err := (CodexAdapter{}).Render(input); err == nil || !strings.Contains(err.Error(), "use RenderDocuments") {
		t.Fatalf("single-document API silently dropped nested outputs: %v", err)
	}
}

func TestCodexAdapterDoesNotBroadenMixedOrNonliteralScopes(t *testing.T) {
	for _, scope := range [][]string{
		{"services/**", "services/api/**/*.go"},
		{"services/*/"},
		{"services/../private/**"},
		{".git/**"},
		{".forge/**"},
		{`services\\api/**`},
	} {
		docs, err := (CodexAdapter{}).RenderDocuments(domain.Input{Project: "sample", Rules: []domain.Rule{{ID: "scoped", Description: "Keep it scoped.", Paths: scope}}})
		if err != nil {
			t.Fatalf("scope %q: %v", scope, err)
		}
		if len(docs) != 1 || docs[0].Path != "AGENTS.md" {
			t.Errorf("scope %q was broadened to nested files: %#v", scope, docs)
		}
		for _, authored := range scope {
			if !strings.Contains(string(docs[0].Content), authored) {
				t.Errorf("scope %q lost authored pattern %q: %s", scope, authored, docs[0].Content)
			}
		}
	}
}
