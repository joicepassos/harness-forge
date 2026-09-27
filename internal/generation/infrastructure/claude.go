package infrastructure

import "harnessforge/internal/generation/domain"

// ClaudeAdapter renders approved HarnessForge instructions for Claude Code.
// Claude Code consumes project instructions from CLAUDE.md; path scopes,
// skill references, and quality gates are emitted by the shared deterministic
// Markdown renderer as guidance (gates are not enforced by this file).
type ClaudeAdapter struct{}

// Render creates a deterministic CLAUDE.md document using the existing
// renderer so rule scopes, skill paths, and gate workspace metadata retain
// their established representation.
func (ClaudeAdapter) Render(input domain.Input) (domain.Document, error) {
	return (Markdown{Agent: "claude"}).Render(input)
}
