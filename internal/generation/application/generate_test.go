package application

import (
	"context"
	"harnessforge/internal/generation/domain"
	harnessdomain "harnessforge/internal/harness/domain"
	"testing"
)

type loader struct{ harness harnessdomain.Harness }

func (l loader) Load(string) (harnessdomain.Harness, error) { return l.harness, nil }

type adapter struct{ input domain.Input }

func (a *adapter) Render(input domain.Input) (domain.Document, error) {
	a.input = input
	return domain.Document{Path: "AGENTS.md", Content: []byte("generated")}, nil
}

type writer struct{ called bool }

func (w *writer) Write(context.Context, string, domain.Document) error { w.called = true; return nil }
func TestGenerateIncludesOnlyApprovedRulesAndQualityCommands(t *testing.T) {
	a, w := &adapter{}, &writer{}
	h := harnessdomain.Harness{Project: harnessdomain.Project{Name: "sample"}, Rules: []harnessdomain.Rule{{ID: "approved", Description: "Keep", Status: "approved"}, {ID: "candidate", Description: "Skip", Status: "candidate"}, {ID: "rejected", Description: "Skip", Status: "rejected"}}, QualityGates: []harnessdomain.QualityGate{{ID: "test", Command: "go test ./..."}}}
	if err := NewGenerate(loader{h}, a, w).Execute(context.Background(), "harness.yaml", "repository"); err != nil {
		t.Fatal(err)
	}
	if !w.called || len(a.input.Rules) != 1 || a.input.Rules[0].ID != "approved" || len(a.input.Commands) != 1 {
		t.Fatalf("%#v", a.input)
	}
}
