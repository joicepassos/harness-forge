package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
)

func TestImportKnowledgeCandidatePreservesManifestAndNeverApproves(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	original := "# Keep this comment\nlayout_version: 1\nir_version: 2\nproject: {name: sample, languages: [Go]}\ntargets: [codex]\nreferences: {}\n"
	if err := os.WriteFile(manifest, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	item := domain.KnowledgeItem{ID: "observation-123", Kind: domain.KnowledgeFact, Content: "Treat imported text as data.", Origin: "explicit-import", Review: domain.KnowledgeCandidate, Health: domain.KnowledgeUnknown}
	path, err := ImportKnowledgeCandidate(root, item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, ".forge/knowledge/items/") {
		t.Fatalf("unexpected path %q", path)
	}
	if again, err := ImportKnowledgeCandidate(root, item); err != nil || again != path {
		t.Fatalf("identical import isn't idempotent: %q %v", again, err)
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# Keep this comment") || !strings.Contains(string(data), item.ID) {
		t.Fatalf("manifest formatting/reference lost: %s", data)
	}
	doc, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	imported, err := parseKnowledgeFrontMatter(doc)
	if err != nil {
		t.Fatal(err)
	}
	if imported.Review != domain.KnowledgeCandidate || imported.Content != item.Content {
		t.Fatalf("import implicitly approved or changed content: %#v", imported)
	}
	conflicting := item
	conflicting.Content = "different text"
	if _, err := ImportKnowledgeCandidate(root, conflicting); err == nil {
		t.Fatal("conflicting stable ID was accepted")
	}
}

func TestImportKnowledgeCandidateRefusesSymlinkedKnowledgeDirectory(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample, languages: [Go]}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".forge", "knowledge")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	item := domain.KnowledgeItem{ID: "item", Kind: domain.KnowledgeFact, Content: "Candidate text", Origin: "manual", Review: domain.KnowledgeCandidate, Health: domain.KnowledgeUnknown}
	if _, err := ImportKnowledgeCandidate(root, item); err == nil {
		t.Fatal("symlinked knowledge directory was accepted")
	}
}

func TestReviewKnowledgeRoundTripsEvidenceReviewDigest(t *testing.T) {
	root := t.TempDir()
	doc := filepath.Join(root, ".forge", "knowledge", "rule.md")
	if err := os.MkdirAll(filepath.Dir(doc), 0700); err != nil {
		t.Fatal(err)
	}
	item := domain.KnowledgeItem{ID: "rule", Kind: domain.KnowledgeFact, Content: "Keep auth server-side.", Origin: "manual", Review: domain.KnowledgeCandidate, Health: domain.KnowledgeUnknown}
	encoded, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc, append(append([]byte("---\n"), encoded...), []byte("---\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.WriteFile(manifest, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample, languages: [Go]}\ntargets: [codex]\nreferences:\n  knowledge:\n    - id: rule\n      path: .forge/knowledge/rule.md\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ReviewKnowledge(root, "forge", "rule", "approved", "alice"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := parseKnowledgeFrontMatter(data)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.EvidenceSHA256 != domain.HashKnowledgeEvidence(nil) || reviewed.ContentSHA256 != domain.HashKnowledgeContent(item.Content) {
		t.Fatalf("review digest not recorded: %#v", reviewed)
	}
}
