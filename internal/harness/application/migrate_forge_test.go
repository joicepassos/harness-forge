package application_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/application"
	"harnessforge/internal/harness/domain"
	"harnessforge/internal/harness/infrastructure"
)

const legacyHarnessForForgeMigration = `version: 2
project:
  name: payment-service
  languages: [Go]
architecture:
  styles: [hexagonal, domain-driven]
rules:
  - id: idempotent-payment
    description: Payment creation is idempotent by request key.
    scope:
      paths: [internal/payment/**]
    origin: human
    status: approved
    review:
      content_sha256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      evidence_sha256: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
    evidence:
      - file: internal/payment/service.go
        kind: symbol
        workspace: backend
        symbol: RequestKey
        quote: same request key returns the original payment
        start_line: 4
        end_line: 4
        sha256: cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
        revision: 0123456789abcdef
skills:
  - id: payment-review
    description: Review payment provider changes
    path: .harness/skills/payment/SKILL.md
    status: approved
    evidence:
      - file: internal/payment/service.go
        kind: symbol
        workspace: backend
        symbol: RequestKey
        quote: same request key returns the original payment
        start_line: 4
        end_line: 4
        sha256: cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
        revision: 0123456789abcdef
quality_gates:
  - id: unit
    command: go test ./...
    workspace: .
    workspaces: [.]
    env: {GOFLAGS: -count=1}
`

func writeLegacyMigrationProject(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, ".harness", "harness.yaml")
	if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(legacyHarnessForForgeMigration), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "payment"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "payment", "service.go"), []byte("// RequestKey\n// same request key returns the original payment\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".harness", "skills", "payment"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".harness", "skills", "payment", "SKILL.md"), []byte("# Payment review\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return root, source
}

func TestPreviewToForgeReportsUnmappedChoicesWithoutWriting(t *testing.T) {
	root, source := writeLegacyMigrationProject(t)
	loader := infrastructure.YAMLLoader{}
	plan, err := application.PreviewToForge(loader, root, source, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Unmapped) != 2 || len(plan.Files) != 0 {
		t.Fatalf("unmapped migration choices were hidden: %#v", plan)
	}
	if _, err := os.Stat(filepath.Join(root, ".forge")); !os.IsNotExist(err) {
		t.Fatal("preview wrote .forge")
	}
}

func TestPreviewToForgePreservesFieldsAndProducesDeterministicPlan(t *testing.T) {
	root, source := writeLegacyMigrationProject(t)
	loader := infrastructure.YAMLLoader{}
	first, err := application.PreviewToForge(loader, root, source, []string{"codex", "claude"}, domain.KnowledgeBusinessRule)
	if err != nil {
		t.Fatal(err)
	}
	second, err := application.PreviewToForge(loader, root, source, []string{"codex", "claude"}, domain.KnowledgeBusinessRule)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Unmapped) != 0 || len(first.Files) != 2 || first.SourceHash != second.SourceHash {
		t.Fatalf("invalid preview: %#v", first)
	}
	for i := range first.Files {
		if first.Files[i] != second.Files[i] {
			t.Fatalf("preview is not deterministic: %#v != %#v", first.Files[i], second.Files[i])
		}
	}
	var manifest domain.Manifest
	for _, file := range first.Files {
		if file.Path == ".forge/forge.yaml" {
			if err := yaml.Unmarshal([]byte(file.Content), &manifest); err != nil {
				t.Fatal(err)
			}
		}
	}
	if manifest.Project.Name != "payment-service" || manifest.Targets[1] != "claude" || manifest.References.Skills[0].ID != "payment-review" || manifest.References.Skills[0].Description != "Review payment provider changes" || manifest.References.Skills[0].Path != ".harness/skills/payment/SKILL.md" || manifest.References.Skills[0].Status != "approved" {
		t.Fatalf("manifest lost legacy fields: %#v", manifest)
	}
	if len(manifest.Architecture.Styles) != 2 || manifest.Architecture.Styles[0] != "hexagonal" || manifest.Architecture.Styles[1] != "domain-driven" {
		t.Fatalf("manifest lost architecture styles: %#v", manifest.Architecture)
	}
	if len(manifest.References.Skills[0].Evidence) != 1 || manifest.References.Skills[0].Evidence[0] != (domain.Evidence{
		File: "internal/payment/service.go", Kind: "symbol", Workspace: "backend", Symbol: "RequestKey",
		Quote: "same request key returns the original payment", StartLine: 4, EndLine: 4,
		SHA256: strings.Repeat("c", 64), Revision: "0123456789abcdef",
	}) {
		t.Fatalf("skill evidence was lost: %#v", manifest.References.Skills[0].Evidence)
	}
	if manifest.QualityGates[0].Command != "go test ./..." || manifest.QualityGates[0].Workspace != "." || len(manifest.QualityGates[0].Workspaces) != 1 || manifest.QualityGates[0].Workspaces[0] != "." || manifest.QualityGates[0].Env["GOFLAGS"] != "-count=1" || manifest.References.Knowledge[0].ID != "idempotent-payment" {
		t.Fatalf("gates or knowledge references lost: %#v", manifest)
	}
	var knowledge domain.KnowledgeItem
	for _, file := range first.Files {
		if strings.HasSuffix(file.Path, ".md") {
			parts := strings.SplitN(file.Content, "---\n", 3)
			if len(parts) != 3 {
				t.Fatalf("invalid knowledge front matter: %s", file.Content)
			}
			if err := yaml.Unmarshal([]byte(parts[1]), &knowledge); err != nil {
				t.Fatal(err)
			}
		}
	}
	if knowledge.ID != "idempotent-payment" || knowledge.Kind != domain.KnowledgeBusinessRule || knowledge.Scope.Paths[0] != "internal/payment/**" || knowledge.Origin != "human" || knowledge.Evidence[0] != (domain.KnowledgeEvidence{
		Path: "internal/payment/service.go", Kind: "symbol", Workspace: "backend", Symbol: "RequestKey",
		Quote: "same request key returns the original payment", StartLine: 4, EndLine: 4,
		SHA256: strings.Repeat("c", 64), Revision: "0123456789abcdef",
	}) || knowledge.Review != domain.KnowledgeCandidate || knowledge.Health != domain.KnowledgeUnknown || knowledge.ContentSHA256 != domain.HashKnowledgeContent("Payment creation is idempotent by request key.") || knowledge.LegacyReviewStatus != "approved" || knowledge.LegacyReview == nil || knowledge.LegacyReview.ContentSHA256 != strings.Repeat("a", 64) || knowledge.LegacyReview.EvidenceSHA256 != strings.Repeat("b", 64) {
		t.Fatalf("rule metadata was not preserved safely: %#v", knowledge)
	}
	if len(first.Warnings) != 1 || !strings.Contains(first.Warnings[0], "candidate") {
		t.Fatalf("approval reset was not disclosed: %#v", first.Warnings)
	}
}

func TestPreviewToForgePreservesEveryMappedLegacyField(t *testing.T) {
	root, source := writeLegacyMigrationProject(t)
	loader := infrastructure.YAMLLoader{}
	legacy, err := loader.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := application.PreviewToForge(loader, root, source, []string{"codex", "claude"}, domain.KnowledgeBusinessRule)
	if err != nil {
		t.Fatal(err)
	}
	manifestFile := planFileContent(t, plan, ".forge/forge.yaml")
	var manifest domain.Manifest
	if err := yaml.Unmarshal([]byte(manifestFile), &manifest); err != nil {
		t.Fatal(err)
	}
	wantManifest := domain.Manifest{
		LayoutVersion: 1,
		IRVersion:     legacy.Version,
		Project:       legacy.Project,
		Architecture:  legacy.Architecture,
		Targets:       []string{"codex", "claude"},
		References: domain.ManifestReferences{
			Knowledge: make([]domain.KnowledgeReference, len(legacy.Rules)),
			Skills:    make([]domain.SkillReference, len(legacy.Skills)),
		},
		QualityGates: append([]domain.QualityGate(nil), legacy.QualityGates...),
	}
	wantKnowledge := make(map[string]domain.KnowledgeItem, len(legacy.Rules))
	for i, rule := range legacy.Rules {
		idHash := sha256.Sum256([]byte(rule.ID))
		itemPath := ".forge/knowledge/items/" + hex.EncodeToString(idHash[:]) + ".md"
		wantManifest.References.Knowledge[i] = domain.KnowledgeReference{ID: rule.ID, Path: itemPath}
		evidence := make([]domain.KnowledgeEvidence, len(rule.Evidence))
		for j, item := range rule.Evidence {
			evidence[j] = domain.KnowledgeEvidence{
				Path: item.File, Workspace: item.Workspace, Kind: item.Kind, Symbol: item.Symbol,
				Quote: item.Quote, StartLine: item.StartLine, EndLine: item.EndLine,
				SHA256: item.SHA256, Revision: item.Revision,
			}
		}
		wantKnowledge[itemPath] = domain.KnowledgeItem{
			ID: rule.ID, Kind: domain.KnowledgeBusinessRule, Scope: rule.Scope,
			Content: rule.Description, Origin: rule.Origin,
			Review: domain.KnowledgeCandidate, Health: domain.KnowledgeUnknown,
			Evidence: evidence, ContentSHA256: domain.HashKnowledgeContent(rule.Description),
			LegacyReviewStatus: rule.Status, LegacyReview: rule.Review,
		}
	}
	for i, skill := range legacy.Skills {
		wantManifest.References.Skills[i] = domain.SkillReference{
			ID: skill.ID, Description: skill.Description, Path: skill.Path, Status: skill.Status,
			Evidence: append([]domain.Evidence(nil), skill.Evidence...),
		}
	}
	if !reflect.DeepEqual(manifest, wantManifest) {
		t.Fatalf("Forge manifest did not preserve every mapped legacy field:\n got: %#v\nwant: %#v", manifest, wantManifest)
	}
	for _, file := range plan.Files {
		want, ok := wantKnowledge[file.Path]
		if !ok {
			continue
		}
		parts := strings.SplitN(file.Content, "---\n", 3)
		if len(parts) != 3 {
			t.Fatalf("invalid knowledge front matter in %s", file.Path)
		}
		var got domain.KnowledgeItem
		if err := yaml.Unmarshal([]byte(parts[1]), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("knowledge migration lost a mapped rule field in %s:\n got: %#v\nwant: %#v", file.Path, got, want)
		}
		delete(wantKnowledge, file.Path)
	}
	if len(wantKnowledge) != 0 {
		t.Fatalf("migration omitted knowledge files for: %#v", wantKnowledge)
	}
}

func TestApplyAndRollbackForgeMigrationProtectEditsAndPreserveSource(t *testing.T) {
	root, source := writeLegacyMigrationProject(t)
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := application.PreviewToForge(infrastructure.YAMLLoader{}, root, source, []string{"codex"}, domain.KnowledgeConvention)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.ApplyForgeMigration(root, plan, plan.PlanSHA256); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(source)
	if err != nil || string(after) != string(original) {
		t.Fatalf("source changed during migration: %v", err)
	}
	project, err := infrastructure.LoadProject(root, "forge")
	if err != nil || project.Manifest == nil || project.Manifest.Project.Name != "payment-service" {
		t.Fatalf("migrated layout did not load: %#v %v", project, err)
	}
	var report map[string]any
	reportBytes, err := os.ReadFile(filepath.Join(root, ".forge", "migration-report.json"))
	if err != nil || json.Unmarshal(reportBytes, &report) != nil {
		t.Fatalf("migration report missing or invalid: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte("manual change"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := application.RollbackForgeMigration(root); err == nil {
		t.Fatal("rollback removed manually edited output")
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(planFileContent(t, plan, ".forge/forge.yaml")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := application.RollbackForgeMigration(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".forge")); !os.IsNotExist(err) {
		t.Fatal("successful rollback left generated layout behind")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("legacy source was not preserved")
	}
}

func TestApplyForgeMigrationRejectsChangedSourceAndRollbackRejectsUnownedDirectory(t *testing.T) {
	root, source := writeLegacyMigrationProject(t)
	plan, err := application.PreviewToForge(infrastructure.YAMLLoader{}, root, source, []string{"codex"}, domain.KnowledgeConvention)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(strings.Replace(legacyHarnessForForgeMigration, "payment-service", "changed-service", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := application.ApplyForgeMigration(root, plan, plan.PlanSHA256); err == nil || !strings.Contains(err.Error(), "changed after preview") {
		t.Fatalf("changed source was migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".forge")); !os.IsNotExist(err) {
		t.Fatal("failed migration created target layout")
	}
	if err := os.WriteFile(source, []byte(legacyHarnessForForgeMigration), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err = application.PreviewToForge(infrastructure.YAMLLoader{}, root, source, []string{"codex"}, domain.KnowledgeConvention)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.ApplyForgeMigration(root, plan, plan.PlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".forge", "manual-empty"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := application.RollbackForgeMigration(root); err == nil || !strings.Contains(err.Error(), "unowned directory") {
		t.Fatalf("rollback removed unowned directory: %v", err)
	}
}

func planFileContent(t *testing.T, plan application.ForgeMigrationPlan, path string) string {
	t.Helper()
	for _, file := range plan.Files {
		if file.Path == path {
			return file.Content
		}
	}
	t.Fatalf("file %q not found in migration plan", path)
	return ""
}
