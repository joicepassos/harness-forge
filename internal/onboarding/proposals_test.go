package onboarding

import (
	"testing"

	llmdomain "harnessforge/internal/llm/domain"
)

func TestCandidatesRequireRepositoryEvidenceAndRemainReviewable(t *testing.T) {
	proposals := Candidates(llmdomain.ArchitectureAnalysis{Patterns: []llmdomain.Pattern{
		{Name: "HTTP middleware", Evidence: []llmdomain.Evidence{{Source: "repository-file:services/auth/main.go#L4", Quote: "Use middleware", Workspace: "services/auth"}}},
		{Name: "Unsupported inference", Evidence: []llmdomain.Evidence{{Source: "model-summary", Quote: "not grounded"}}},
	}})
	if len(proposals) != 1 || proposals[0].ID != "http-middleware" {
		t.Fatalf("proposals = %#v", proposals)
	}
	if proposals[0].Evidence[0].Workspace != "services/auth" || proposals[0].Evidence[0].File != "services/auth/main.go" {
		t.Fatalf("evidence = %#v", proposals[0].Evidence[0])
	}
}
