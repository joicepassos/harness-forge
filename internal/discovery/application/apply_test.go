package application

import (
	"harnessforge/internal/discovery/domain"
	"testing"
)

type store struct{ called bool }

func (s *store) Apply(string, string, domain.Proposal) error { s.called = true; return nil }
func TestApplyRequiresApprovalAndValidEvidence(t *testing.T) {
	s := &store{}
	proposal := domain.Proposal{ID: "use-ports", Description: "Use ports", Evidence: []domain.Evidence{{File: "port.go", Symbol: "Port"}}}
	if err := NewApply(s).Execute("repository", "harness", proposal, false); err == nil || s.called {
		t.Fatal("unapproved proposal applied")
	}
	proposal.ID = "../unsafe"
	if err := NewApply(s).Execute("repository", "harness", proposal, true); err == nil || s.called {
		t.Fatal("unsafe proposal applied")
	}
	proposal.ID = "use-ports"
	if err := NewApply(s).Execute("repository", "harness", proposal, true); err != nil || !s.called {
		t.Fatal(err)
	}
}
