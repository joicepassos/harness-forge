package application

import (
	"context"
	"harnessforge/internal/generation/domain"
	harnessdomain "harnessforge/internal/harness/domain"
	"strings"
	"testing"
)

type loader struct{ harness harnessdomain.Harness }

func (l loader) Load(string) (harnessdomain.Harness, error) { return l.harness, nil }

type evidenceFingerprinter string

func (f evidenceFingerprinter) Fingerprint(string, string, []harnessdomain.Evidence) (string, error) {
	return string(f), nil
}

type adapter struct{ input domain.Input }

func (a *adapter) Render(input domain.Input) (domain.Document, error) {
	a.input = input
	return domain.Document{Path: "AGENTS.md", Content: []byte("generated")}, nil
}

type writer struct{ called bool }

func (w *writer) Write(context.Context, string, domain.Document) error { w.called = true; return nil }
func TestGeneratePreservesSkillsAndStructuredQualityGates(t *testing.T) {
	a, w := &adapter{}, &writer{}
	h := harnessdomain.Harness{Project: harnessdomain.Project{Name: "sample"}, Rules: []harnessdomain.Rule{{ID: "approved", Description: "Keep", Status: "approved"}, {ID: "candidate", Description: "Skip", Status: "candidate"}, {ID: "rejected", Description: "Skip", Status: "rejected"}}, Skills: []harnessdomain.Skill{{ID: "backend", Description: "Backend conventions", Path: "skills/backend/SKILL.md"}, {ID: "draft", Description: "Not published", Status: "candidate"}}, QualityGates: []harnessdomain.QualityGate{{ID: "test", Command: "go test ./...", Workspace: "services/api", Workspaces: []string{"services/api", "libs/core"}}}}
	if err := NewGenerate(loader{h}, a, w).Execute(context.Background(), "harness.yaml", "repository"); err != nil {
		t.Fatal(err)
	}
	if !w.called || len(a.input.Rules) != 1 || a.input.Rules[0].ID != "approved" || len(a.input.Skills) != 1 || a.input.Skills[0].Path != "skills/backend/SKILL.md" || len(a.input.Gates) != 1 {
		t.Fatalf("%#v", a.input)
	}
	gate := a.input.Gates[0]
	if gate.Workspace != "services/api" || len(gate.Workspaces) != 2 || gate.Workspaces[1] != "libs/core" {
		t.Fatalf("gate workspace lost: %#v", gate)
	}
}

func TestGenerateRejectsReviewedRuleChangedAfterApproval(t *testing.T) {
	rule := harnessdomain.Rule{ID: "reviewed", Description: "Keep this rule", Origin: "human", Status: "approved", Evidence: []harnessdomain.Evidence{{File: "architecture.md", Quote: "stable"}}}
	contentHash, err := harnessdomain.RuleContentHash(rule)
	if err != nil {
		t.Fatal(err)
	}
	const evidenceHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	rule.Review = &harnessdomain.ReviewRecord{ContentSHA256: contentHash, EvidenceSHA256: evidenceHash}
	rule.Description = "Changed after approval"
	h := harnessdomain.Harness{Project: harnessdomain.Project{Name: "sample"}, Rules: []harnessdomain.Rule{rule}}
	w := &writer{}
	err = NewGenerate(loader{h}, &adapter{}, w, evidenceFingerprinter(evidenceHash)).Execute(context.Background(), "harness.yaml", "repository")
	if err == nil || !strings.Contains(err.Error(), "changed after review") {
		t.Fatalf("changed approved content was published: %v", err)
	}
	if w.called {
		t.Fatal("writer ran for a rule whose review hash is stale")
	}
}

func TestGenerateRejectsEvidenceChangedAfterApproval(t *testing.T) {
	rule := harnessdomain.Rule{ID: "reviewed", Description: "Keep this rule", Origin: "human", Status: "approved", Evidence: []harnessdomain.Evidence{{File: "architecture.md", Quote: "stable"}}}
	contentHash, err := harnessdomain.RuleContentHash(rule)
	if err != nil {
		t.Fatal(err)
	}
	const reviewedEvidenceHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	rule.Review = &harnessdomain.ReviewRecord{ContentSHA256: contentHash, EvidenceSHA256: reviewedEvidenceHash}
	h := harnessdomain.Harness{Project: harnessdomain.Project{Name: "sample"}, Rules: []harnessdomain.Rule{rule}}
	w := &writer{}
	changedEvidenceHash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	err = NewGenerate(loader{h}, &adapter{}, w, evidenceFingerprinter(changedEvidenceHash)).Execute(context.Background(), "harness.yaml", "repository")
	if err == nil || !strings.Contains(err.Error(), "evidence changed after review") {
		t.Fatalf("changed evidence was published: %v", err)
	}
	if w.called {
		t.Fatal("writer ran for evidence that changed after review")
	}
}
