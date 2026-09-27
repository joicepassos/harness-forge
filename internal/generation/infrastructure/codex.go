package infrastructure

import "harnessforge/internal/generation/domain"

// CodexAdapter renders the canonical instruction file used by Codex.
// Codex discovers AGENTS.md files along the directory hierarchy, so rule
// scopes are retained in the rendered content by the shared Markdown renderer.
type CodexAdapter struct{}

// Render produces the root AGENTS.md document for a project.
func (CodexAdapter) Render(input domain.Input) (domain.Document, error) {
	return (Markdown{Agent: "codex"}).Render(input)
}
