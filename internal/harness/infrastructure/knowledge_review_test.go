package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
	"go.yaml.in/yaml/v3"
	harnessdomain "harnessforge/internal/harness/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewKnowledgePromotesWithReviewerHashAndPreservesMarkdown(t *testing.T) {
	root := t.TempDir()
	evidencePath := filepath.Join(root, "docs", "architecture.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0700); err != nil {
		t.Fatal(err)
	}
	evidence := []byte("Use an explicit repository boundary.")
	if err := os.WriteFile(evidencePath, evidence, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(evidence)
	item := harnessdomain.KnowledgeItem{ID: "boundary", Kind: harnessdomain.KnowledgeConvention, Content: "Keep file access within repository boundaries.", Origin: "human", Review: harnessdomain.KnowledgeCandidate, Health: harnessdomain.KnowledgeUnknown, Evidence: []harnessdomain.KnowledgeEvidence{{Path: "docs/architecture.md", Quote: string(evidence), SHA256: hex.EncodeToString(sum[:])}}}
	writeForgeKnowledgeReviewFixture(t, root, item)
	if err := ReviewKnowledge(root, "forge", "boundary", "approved", "alice"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".forge", "knowledge", "boundary.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "---\n\n# Human body\nKeep this section.\n") {
		t.Fatalf("markdown body changed: %q", data)
	}
	parsed, err := parseReviewKnowledge(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Review != harnessdomain.KnowledgeApproved || parsed.Reviewer != "alice" || parsed.Health != harnessdomain.KnowledgeVerified || parsed.ContentSHA256 != harnessdomain.HashKnowledgeContent(item.Content) {
		t.Fatalf("review record incomplete: %#v", parsed)
	}
}

func TestReviewKnowledgeRefusesChangedEvidenceAndMissingReviewer(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "source.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	item := harnessdomain.KnowledgeItem{ID: "rule", Kind: harnessdomain.KnowledgeFact, Content: "Original rule content", Origin: "human", Review: harnessdomain.KnowledgeCandidate, Health: harnessdomain.KnowledgeUnknown, Evidence: []harnessdomain.KnowledgeEvidence{{Path: "docs/source.md", Quote: "original evidence"}}}
	writeForgeKnowledgeReviewFixture(t, root, item)
	if err := ReviewKnowledge(root, "forge", "rule", "approved", "alice"); err == nil || !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("changed evidence accepted: %v", err)
	}
	if err := ReviewKnowledge(root, "forge", "rule", "rejected", ""); err == nil || !strings.Contains(err.Error(), "reviewer") {
		t.Fatalf("missing reviewer accepted: %v", err)
	}
}

func TestReviewKnowledgeRejectsSecondDecision(t *testing.T) {
	root := t.TempDir()
	item := harnessdomain.KnowledgeItem{ID: "decision", Kind: harnessdomain.KnowledgeFact, Content: "Keep the boundary explicit.", Origin: "human", Review: harnessdomain.KnowledgeCandidate, Health: harnessdomain.KnowledgeUnknown}
	writeForgeKnowledgeReviewFixture(t, root, item)
	if err := ReviewKnowledge(root, "forge", item.ID, "approved", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := ReviewKnowledge(root, "forge", item.ID, "rejected", "bob"); err == nil || !strings.Contains(err.Error(), "only candidate knowledge") {
		t.Fatalf("second review decision accepted: %v", err)
	}
}

func writeForgeKnowledgeReviewFixture(t *testing.T, root string, item harnessdomain.KnowledgeItem) {
	t.Helper()
	doc := filepath.Join(root, ".forge", "knowledge", "boundary.md")
	if item.ID != "boundary" {
		doc = filepath.Join(root, ".forge", "knowledge", item.ID+".md")
	}
	if err := os.MkdirAll(filepath.Dir(doc), 0700); err != nil {
		t.Fatal(err)
	}
	encoded, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc, append(append([]byte("---\n"), encoded...), []byte("---\n\n# Human body\nKeep this section.\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := "layout_version: 1\nir_version: 2\nproject: {name: sample, languages: [Go]}\ntargets: [codex]\nreferences:\n  knowledge:\n    - id: " + item.ID + "\n      path: .forge/knowledge/" + item.ID + ".md\n"
	path := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
}

func parseReviewKnowledge(data []byte) (harnessdomain.KnowledgeItem, error) {
	text := strings.TrimSpace(string(data))
	parts := strings.SplitN(strings.TrimPrefix(text, "---"), "---", 2)
	var item harnessdomain.KnowledgeItem
	err := yaml.Unmarshal([]byte(parts[0]), &item)
	return item, err
}
