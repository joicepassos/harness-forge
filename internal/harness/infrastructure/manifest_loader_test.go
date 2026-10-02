package infrastructure

import (
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
	"harnessforge/internal/inputlimits"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validForgeManifest = `layout_version: 1
ir_version: 2
project:
  name: sample
  languages: [Go]
architecture:
  styles: [hexagonal]
targets: [codex, claude]
references:
  knowledge:
    - id: architecture
      path: .forge/knowledge/architecture.md
    - id: approved-api
      path: .forge/knowledge/approved-api.md
  skills:
    - id: review
      description: Review skill
      path: .forge/skills/review/SKILL.md
      status: approved
      evidence:
        - file: docs/architecture.md
          quote: ports and adapters
quality_gates:
  - id: tests
    command: go test ./...
    workspace: .
    workspaces: [.]
    env: {GOFLAGS: -count=1}
policies:
  - id: no-network
    description: Do not access external networks
    capability: advisory
    executor: instruction-text
`

func TestManifestLoaderValidatesOfflineAndPreservesRelativeReferences(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(validForgeManifest), 0600); err != nil {
		t.Fatal(err)
	}
	candidate := domain.KnowledgeItem{
		ID: "architecture", Kind: domain.KnowledgeConvention,
		Scope: domain.Scope{Paths: []string{"internal/**"}}, Keywords: []string{"ports"},
		Content: "Keep adapters at the boundary.", Origin: "human-review", Review: domain.KnowledgeCandidate,
		Health: domain.KnowledgeUnknown, Evidence: []domain.KnowledgeEvidence{{Path: "docs/architecture.md", Kind: "documentation", Quote: "ports and adapters", StartLine: 4, EndLine: 4, Revision: "abc123"}},
	}
	approved := domain.KnowledgeItem{
		ID: "approved-api", Kind: domain.KnowledgeDecision,
		Scope: domain.Scope{Paths: []string{"internal/api/**"}}, Keywords: []string{"pagination"},
		Content: "Use cursor pagination.", Origin: "incident-42", Review: domain.KnowledgeApproved,
		Health: domain.KnowledgeVerified, Reviewer: "alice", Evidence: []domain.KnowledgeEvidence{{Path: "docs/api.md", Kind: "documentation", Symbol: "ListItems", Quote: "Use a cursor.", StartLine: 9, EndLine: 9, SHA256: strings.Repeat("a", 64), Revision: "def456"}},
	}
	approved.ContentSHA256 = domain.HashKnowledgeContent(approved.Content)
	approved.EvidenceSHA256 = domain.HashKnowledgeEvidence([]string{"docs/api.md:" + strings.Repeat("a", 64)})
	approved.ReviewMetadataSHA256 = domain.HashKnowledgeReviewMetadata(approved)
	writeKnowledgeDocument(t, root, candidate)
	writeKnowledgeDocument(t, root, approved)
	manifest, err := (ManifestLoader{}).Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.LayoutVersion != 1 || manifest.IRVersion != 2 || manifest.References.Knowledge[0].Path != ".forge/knowledge/architecture.md" {
		t.Fatalf("manifest fields were not preserved: %#v", manifest)
	}
	layout, err := ResolveLayout(root, "forge")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := layout.ResolveReference(manifest.References.Knowledge[0].Path)
	if err != nil || resolved != filepath.Join(root, ".forge", "knowledge", "architecture.md") {
		t.Fatalf("relative reference resolved to %q: %v", resolved, err)
	}
	project, err := LoadProject(root, "forge")
	if err != nil || project.Manifest == nil || project.Harness.Version != 2 || project.Harness.Project.Name != "sample" {
		t.Fatalf("project load=%#v err=%v", project, err)
	}
	if len(project.Harness.Architecture.Styles) != 1 || project.Harness.Architecture.Styles[0] != "hexagonal" || len(project.Harness.Skills) != 1 || project.Harness.Skills[0].Status != "approved" || project.Harness.Skills[0].Evidence[0].Quote != "ports and adapters" || project.Harness.QualityGates[0].Env["GOFLAGS"] != "-count=1" {
		t.Fatalf("compatibility projection lost shared fields: %#v", project.Harness)
	}
	if project.Manifest.LayoutVersion != 1 || len(project.Manifest.Targets) != 2 || len(project.Manifest.Policies) != 1 || project.Manifest.Policies[0].ID != "no-network" || len(project.Manifest.References.Knowledge) != 2 {
		t.Fatalf("Forge-only fields were not retained in the manifest: %#v", project.Manifest)
	}
	if len(project.Knowledge) != 2 || project.Knowledge[0].ID != candidate.ID || project.Knowledge[1].ID != approved.ID {
		t.Fatalf("referenced typed knowledge was not loaded: %#v", project.Knowledge)
	}
	gotCandidate, gotApproved := project.Knowledge[0], project.Knowledge[1]
	if gotCandidate.Kind != candidate.Kind || gotCandidate.Scope.Paths[0] != "internal/**" || gotCandidate.Origin != candidate.Origin || gotCandidate.Review != domain.KnowledgeCandidate || gotCandidate.Health != domain.KnowledgeUnknown || gotCandidate.Evidence[0].Quote != candidate.Evidence[0].Quote {
		t.Fatalf("candidate fields were not preserved: %#v", gotCandidate)
	}
	if gotApproved.Kind != approved.Kind || gotApproved.Scope.Paths[0] != "internal/api/**" || gotApproved.Origin != approved.Origin || gotApproved.Review != domain.KnowledgeApproved || gotApproved.Health != domain.KnowledgeVerified || gotApproved.Reviewer != "alice" || gotApproved.ContentSHA256 != approved.ContentSHA256 || gotApproved.EvidenceSHA256 != approved.EvidenceSHA256 || gotApproved.ReviewMetadataSHA256 != approved.ReviewMetadataSHA256 || gotApproved.Evidence[0] != approved.Evidence[0] {
		t.Fatalf("approved fields were not preserved: %#v", gotApproved)
	}
	if len(project.Harness.Rules) != 0 {
		t.Fatalf("Forge knowledge was flattened into legacy rules: %#v", project.Harness.Rules)
	}
	project.Harness.Architecture.Styles[0] = "mutated"
	project.Harness.Skills[0].Evidence[0].Quote = "mutated"
	project.Harness.QualityGates[0].Workspaces[0] = "mutated"
	project.Harness.QualityGates[0].Env["GOFLAGS"] = "mutated"
	if project.Manifest.Architecture.Styles[0] != "hexagonal" || project.Manifest.References.Skills[0].Evidence[0].Quote != "ports and adapters" || project.Manifest.QualityGates[0].Workspaces[0] != "." || project.Manifest.QualityGates[0].Env["GOFLAGS"] != "-count=1" {
		t.Fatal("compatibility projection shares mutable fields with the Forge manifest")
	}
}

func writeKnowledgeDocument(t *testing.T, root string, item domain.KnowledgeItem) {
	t.Helper()
	data, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".forge", "knowledge", item.ID+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if int64(len(data)) > inputlimits.HarnessYAMLBytes {
		t.Fatal("fixture unexpectedly exceeds Forge knowledge limit")
	}
	if err := os.WriteFile(path, []byte("---\n"+string(data)+"---\n\nbody is retained as inert text\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadProjectRejectsKnowledgeDocumentIDMismatch(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(validForgeManifest), 0600); err != nil {
		t.Fatal(err)
	}
	wrong := domain.KnowledgeItem{
		ID: "different-id", Kind: domain.KnowledgeFact, Content: "A fact.",
		Origin: "test", Review: domain.KnowledgeCandidate, Health: domain.KnowledgeUnknown,
	}
	data, err := yaml.Marshal(wrong)
	if err != nil {
		t.Fatal(err)
	}
	knowledgePath := filepath.Join(root, ".forge", "knowledge", "architecture.md")
	if err := os.MkdirAll(filepath.Dir(knowledgePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(knowledgePath, []byte("---\n"+string(data)+"---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProject(root, "forge"); err == nil || !strings.Contains(err.Error(), `knowledge reference "architecture" points to item "different-id"`) {
		t.Fatalf("ID mismatch should fail closed, got %v", err)
	}
}

func TestManifestLoaderRejectsUnknownFieldsNullAndMultipleDocuments(t *testing.T) {
	tests := []struct{ name, content, want string }{
		{"unknown", validForgeManifest + "secret: value\n", "unknown field"},
		{"null", strings.Replace(validForgeManifest, "targets: [codex, claude]", "targets: null", 1), "null is not allowed"},
		{"multiple", validForgeManifest + "---\n" + validForgeManifest, "one YAML document"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "forge.yaml")
			if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := (ManifestLoader{}).Load(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadProjectRejectsCoexistingLayoutsWithoutSelection(t *testing.T) {
	root := t.TempDir()
	harness := filepath.Join(root, ".harness", "harness.yaml")
	forge := filepath.Join(root, ".forge", "forge.yaml")
	for path, content := range map[string]string{
		harness: "version: 1\nproject: {name: sample}\n",
		forge:   validForgeManifest,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := LoadProject(root, ""); err == nil || !strings.Contains(err.Error(), "select --layout") {
		t.Fatalf("coexisting layouts did not require selection: %v", err)
	}
}

func TestYAMLLoaderLoadsQualityGateEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harness.yaml")
	content := "version: 2\nproject: {name: sample}\nquality_gates:\n  - id: tests\n    command: go test ./...\n    env:\n      GOFLAGS: -count=1\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	h, err := (YAMLLoader{}).Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.QualityGates[0].Env["GOFLAGS"]; got != "-count=1" {
		t.Fatalf("gate env = %q", got)
	}
}
