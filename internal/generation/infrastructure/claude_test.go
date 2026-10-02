package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"harnessforge/internal/generation/domain"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestClaudeAdapterRendersProjectInstructionsDeterministically(t *testing.T) {
	input := domain.Input{
		Project: "sample",
		Rules: []domain.Rule{
			{ID: "z-rule", Description: "Last rule"},
			{ID: "a-rule", Description: "Scoped rule", Paths: []string{"services/api/**", "libs/core/**"}},
		},
		Skills: []domain.Skill{{ID: "api", Description: "API conventions", Path: ".harness/skills/api/SKILL.md"}},
		Gates:  []domain.QualityGate{{ID: "api-tests", Command: "go test ./...", Workspace: "services/api", Workspaces: []string{"services/api", "libs/core"}}},
	}
	adapter := ClaudeAdapter{}
	first, err := adapter.RenderDocuments(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.RenderDocuments(input)
	if err != nil {
		t.Fatal(err)
	}
	if first[0].Path != "CLAUDE.md" || len(first) != 2 {
		t.Fatalf("documents = %#v, want CLAUDE.md and one scoped rule", first)
	}
	if !bytes.Equal(first[0].Content, second[0].Content) || !bytes.Equal(first[1].Content, second[1].Content) {
		t.Fatal("Claude output is not deterministic")
	}
	for _, want := range []string{
		"# sample agent instructions", "[z-rule] Last rule",
		".harness/skills/api/SKILL.md", "api-tests", "go test ./...", "workspace: `services/api`",
		"workspaces: `services/api`, `libs/core`",
	} {
		if !strings.Contains(string(first[0].Content), want) {
			t.Errorf("CLAUDE.md omits %q:\n%s", want, first[0].Content)
		}
	}
	if strings.Contains(string(first[0].Content), "Scoped rule") || !strings.Contains(string(first[1].Content), "Scoped rule") {
		t.Fatal("scoped rule was not separated from CLAUDE.md")
	}
	if strings.Index(string(first[0].Content), "[global]") > strings.Index(string(first[0].Content), "[z-rule]") {
		t.Fatal("global rules are not emitted in stable ID order")
	}
}

func TestClaudeAdapterEmitsNativeScopedRulesAndYamlFrontmatter(t *testing.T) {
	input := domain.Input{
		Project: "sample",
		Rules: []domain.Rule{
			{ID: "global", Description: "Keep changes focused."},
			{ID: "api", Description: "Use API conventions.", Paths: []string{"services/api/**"}},
			{ID: "service/rules", Description: "Use service conventions.", Paths: []string{"services/**", "services/api/**/*.go"}},
		},
	}
	first, err := (ClaudeAdapter{}).RenderDocuments(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := (ClaudeAdapter{}).RenderDocuments(input)
	if err != nil {
		t.Fatal(err)
	}
	reordered := input
	reordered.Rules = append([]domain.Rule(nil), input.Rules...)
	reordered.Rules[2].Paths = []string{"services/api/**/*.go", "services/**"}
	reorderedPaths, err := (ClaudeAdapter{}).RenderDocuments(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 || len(second) != 3 || first[0].Path != "CLAUDE.md" {
		t.Fatalf("Claude outputs = %#v, want CLAUDE.md plus two rule files", first)
	}
	for i := range first {
		if first[i].Path != second[i].Path || !bytes.Equal(first[i].Content, second[i].Content) {
			t.Fatalf("Claude output %d is not deterministic: %#v / %#v", i, first[i], second[i])
		}
		if !bytes.Equal(first[i].Content, reorderedPaths[i].Content) {
			t.Fatalf("Claude output %d depends on glob order", i)
		}
	}
	root := string(first[0].Content)
	if !strings.Contains(root, "[global] Keep changes focused. (global)") || strings.Contains(root, "Use service conventions") || strings.Contains(root, "Use API conventions") {
		t.Fatalf("CLAUDE.md must contain globals only, content:\n%s", root)
	}
	for i, rule := range input.Rules[1:] {
		file := first[i+1]
		nameHash := sha256.Sum256([]byte(rule.ID))
		wantPath := fmt.Sprintf(".claude/rules/%x.md", nameHash[:])
		if file.Path != wantPath {
			t.Fatalf("rule output path = %q, want %q", file.Path, wantPath)
		}
		content := string(file.Content)
		if !strings.HasPrefix(content, "---\n") || !strings.Contains(content, "\n"+marker+"\n") || !strings.Contains(content, "# Approved rule: "+rule.ID) || !strings.Contains(content, rule.Description) {
			t.Fatalf("scoped rule output is missing frontmatter or content:\n%s", content)
		}
		parts := strings.SplitN(strings.TrimPrefix(content, "---\n"), "---\n", 2)
		if len(parts) != 2 {
			t.Fatalf("invalid YAML frontmatter:\n%s", content)
		}
		var parsed struct {
			Paths []string `yaml:"paths"`
		}
		if err := yaml.Unmarshal([]byte(parts[0]), &parsed); err != nil {
			t.Fatalf("frontmatter YAML did not round-trip: %v", err)
		}
		wantPaths := append([]string(nil), rule.Paths...)
		if len(parsed.Paths) != len(wantPaths) {
			t.Fatalf("frontmatter paths = %#v, want %#v", parsed.Paths, wantPaths)
		}
		for j := range wantPaths {
			found := false
			for _, path := range parsed.Paths {
				if path == wantPaths[j] {
					found = true
				}
			}
			if !found {
				t.Errorf("frontmatter widened/dropped path %q: %#v", wantPaths[j], parsed.Paths)
			}
		}
	}
	if _, err := (ClaudeAdapter{}).Render(input); err == nil || !strings.Contains(err.Error(), "use RenderDocuments") {
		t.Fatalf("legacy Render silently discarded path rules: %v", err)
	}
}
