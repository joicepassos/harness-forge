package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"

	"go.yaml.in/yaml/v3"
	"harnessforge/internal/generation/domain"
)

// ClaudeAdapter renders approved HarnessForge instructions for Claude Code.
// Claude Code consumes global instructions from CLAUDE.md and path-scoped
// instructions from .claude/rules files with native paths frontmatter.
type ClaudeAdapter struct{}

// Render preserves the legacy single-document API. Scoped inputs need the
// multi-document API so no native path rule is silently discarded.
func (ClaudeAdapter) Render(input domain.Input) (domain.Document, error) {
	documents, err := (ClaudeAdapter{}).RenderDocuments(input)
	if err != nil {
		return domain.Document{}, err
	}
	if len(documents) != 1 {
		return domain.Document{}, fmt.Errorf("Claude input requires scoped .claude/rules outputs; use RenderDocuments")
	}
	return documents[0], nil
}

// RenderDocuments emits global rules and project-wide context in CLAUDE.md,
// with one deterministic, natively scoped rule file per path-scoped rule.
func (ClaudeAdapter) RenderDocuments(input domain.Input) ([]domain.Document, error) {
	rootInput := input
	rootInput.Rules = nil
	var scoped []domain.Rule
	for _, rule := range input.Rules {
		if len(rule.Paths) == 0 {
			rootInput.Rules = append(rootInput.Rules, rule)
		} else {
			scoped = append(scoped, rule)
		}
	}
	root, err := (Markdown{Agent: "claude"}).Render(rootInput)
	if err != nil {
		return nil, err
	}
	documents := []domain.Document{root}
	sort.Slice(scoped, func(i, j int) bool { return scoped[i].ID < scoped[j].ID })
	for _, rule := range scoped {
		paths := append([]string(nil), rule.Paths...)
		sort.Strings(paths)
		nameHash := sha256.Sum256([]byte(rule.ID))
		outputPath := fmt.Sprintf(".claude/rules/%x.md", nameHash[:])
		frontMatter, err := yaml.Marshal(struct {
			Paths []string `yaml:"paths"`
		}{Paths: paths})
		if err != nil {
			return nil, fmt.Errorf("encode Claude rule %q paths: %w", rule.ID, err)
		}
		var content bytes.Buffer
		content.WriteString("---\n")
		content.Write(frontMatter)
		content.WriteString("---\n")
		fmt.Fprintf(&content, "\n%s\n# Approved rule: %s\n\n%s\n", marker, rule.ID, rule.Description)
		documents = append(documents, domain.Document{Path: outputPath, Content: content.Bytes()})
	}
	return documents, nil
}
