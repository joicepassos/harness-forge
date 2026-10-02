package infrastructure

import (
	"fmt"

	"harnessforge/internal/generation/domain"
)

// CodexAdapter renders Codex project instructions. Directory subtree rule
// scopes can be emitted as nested AGENTS.md files; arbitrary file globs remain
// advisory because Codex discovers instruction files by CWD hierarchy.
type CodexAdapter struct{}

// Render produces the root AGENTS.md document for a project.
func (CodexAdapter) Render(input domain.Input) (domain.Document, error) {
	documents, err := (CodexAdapter{}).RenderDocuments(input)
	if err != nil {
		return domain.Document{}, err
	}
	if len(documents) != 1 {
		return domain.Document{}, fmt.Errorf("Codex input requires nested AGENTS.md outputs; use RenderDocuments")
	}
	return documents[0], nil
}
