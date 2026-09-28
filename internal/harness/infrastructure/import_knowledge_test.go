package infrastructure

import (
	"bytes"
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
	conflicting.Origin = "different-import-origin"
	if _, err := ImportKnowledgeCandidate(root, conflicting); err == nil {
		t.Fatal("same ID with different provenance was accepted as idempotent")
	}
	conflicting = item
	conflicting.Content = "different text"
	if _, err := ImportKnowledgeCandidate(root, conflicting); err == nil {
		t.Fatal("conflicting stable ID was accepted")
	}
	differentSource := item
	differentSource.ID = "observation-456"
	differentSource.Origin = "another-explicit-source"
	otherPath, err := ImportKnowledgeCandidate(root, differentSource)
	if err != nil || otherPath == path {
		t.Fatalf("explicit import with distinct provenance was collapsed: path=%q err=%v", otherPath, err)
	}
}

func TestImportKnowledgeCandidateDeduplicatesEquivalentPublicationWithoutEditingAuditTrail(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample, languages: [Go]}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first := domain.KnowledgeItem{
		ID: "observation-first", Kind: domain.KnowledgeConvention,
		Scope:   domain.Scope{Paths: []string{"services/api/**"}},
		Content: "Use cursor pagination.", Origin: "local-observation:first; reviewer:alice",
		Review: domain.KnowledgeCandidate, Health: domain.KnowledgeUnknown,
		Evidence: []domain.KnowledgeEvidence{{Path: "docs/api.md", Quote: "cursor pagination", SHA256: strings.Repeat("a", 64)}},
	}
	ref, err := ImportKnowledgeCandidateDeduplicatingEquivalent(root, first)
	if err != nil {
		t.Fatal(err)
	}
	documentPath := filepath.Join(root, filepath.FromSlash(ref))
	beforeDocument, err := os.ReadFile(documentPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = "observation-second"
	second.Origin = "local-observation:second; reviewer:bob; revision:later"
	got, err := ImportKnowledgeCandidateDeduplicatingEquivalent(root, second)
	if err != nil {
		t.Fatal(err)
	}
	if got != ref {
		t.Fatalf("equivalent publication returned %q, want existing reference %q", got, ref)
	}
	afterDocument, err := os.ReadFile(documentPath)
	if err != nil {
		t.Fatal(err)
	}
	afterManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeDocument, afterDocument) || !bytes.Equal(beforeManifest, afterManifest) {
		t.Fatal("deduplication changed the existing knowledge audit record or manifest")
	}
	stored, err := parseKnowledgeFrontMatter(afterDocument)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != first.ID || stored.Origin != first.Origin || stored.Review != domain.KnowledgeCandidate {
		t.Fatalf("deduplication overwrote or approved the existing item: %#v", stored)
	}
}

func TestImportKnowledgeCandidateDedupPreservesExistingApproval(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first := domain.KnowledgeItem{ID: "approved-first", Kind: domain.KnowledgeFact, Content: "The service uses cursor pagination.", Origin: "first-reviewed-observation", Review: domain.KnowledgeCandidate, Health: domain.KnowledgeUnknown}
	ref, err := ImportKnowledgeCandidateDeduplicatingEquivalent(root, first)
	if err != nil {
		t.Fatal(err)
	}
	if err := ReviewKnowledge(root, "forge", first.ID, "approved", "alice"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(ref))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = "approved-second"
	second.Origin = "second-reviewed-observation"
	got, err := ImportKnowledgeCandidateDeduplicatingEquivalent(root, second)
	if err != nil {
		t.Fatal(err)
	}
	if got != ref {
		t.Fatalf("equivalent publication returned %q, want existing reference %q", got, ref)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("deduplicating an approved item changed its approval or audit record")
	}
	stored, err := parseKnowledgeFrontMatter(after)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Review != domain.KnowledgeApproved || stored.Reviewer != "alice" || stored.ID != first.ID || stored.Origin != first.Origin {
		t.Fatalf("existing approval or provenance was changed: %#v", stored)
	}
}

func TestImportKnowledgeCandidateDoesNotDeduplicateDifferentEvidenceOrScope(t *testing.T) {
	for _, mutate := range []struct {
		name string
		edit func(*domain.KnowledgeItem)
	}{
		{name: "scope", edit: func(item *domain.KnowledgeItem) { item.Scope.Paths = []string{"services/web/**"} }},
		{name: "evidence", edit: func(item *domain.KnowledgeItem) { item.Evidence[0].Quote = "offset pagination" }},
		{name: "kind", edit: func(item *domain.KnowledgeItem) { item.Kind = domain.KnowledgeFact }},
		{name: "keywords", edit: func(item *domain.KnowledgeItem) { item.Keywords = []string{"page-token"} }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := filepath.Join(root, ".forge", "forge.yaml")
			if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifest, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			first := domain.KnowledgeItem{ID: "first", Kind: domain.KnowledgeConvention, Scope: domain.Scope{Paths: []string{"services/api/**"}}, Content: "Use cursor pagination.", Origin: "first", Review: domain.KnowledgeCandidate, Health: domain.KnowledgeUnknown, Evidence: []domain.KnowledgeEvidence{{Path: "docs/api.md", Quote: "cursor pagination"}}}
			if _, err := ImportKnowledgeCandidateDeduplicatingEquivalent(root, first); err != nil {
				t.Fatal(err)
			}
			second := first
			second.ID = "second"
			second.Origin = "second"
			mutate.edit(&second)
			if _, err := ImportKnowledgeCandidateDeduplicatingEquivalent(root, second); err != nil {
				t.Fatalf("non-equivalent publication was incorrectly treated as a duplicate: %v", err)
			}
			project, err := LoadProject(root, "forge")
			if err != nil {
				t.Fatal(err)
			}
			if len(project.Manifest.References.Knowledge) != 2 {
				t.Fatalf("expected both non-equivalent items to remain referenced, got %d", len(project.Manifest.References.Knowledge))
			}
		})
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
	if reviewed.EvidenceSHA256 != domain.HashKnowledgeEvidence(nil) || reviewed.ContentSHA256 != domain.HashKnowledgeContent(item.Content) || reviewed.ReviewMetadataSHA256 != domain.HashKnowledgeReviewMetadata(reviewed) {
		t.Fatalf("review digest not recorded: %#v", reviewed)
	}
}
