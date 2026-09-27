package domain

import (
	"strings"
	"testing"
)

func candidateKnowledge() KnowledgeItem {
	return KnowledgeItem{
		ID: "api-pagination", Kind: KnowledgeConvention,
		Scope:   Scope{Paths: []string{"services/api/**"}},
		Content: "API list endpoints use cursor pagination.", Origin: "repository",
		Review: KnowledgeCandidate, Health: KnowledgeVerified,
		Evidence: []KnowledgeEvidence{{Path: "services/api/README.md", Kind: "documentation", Quote: "Use cursor pagination.", StartLine: 12, EndLine: 12}},
	}
}

func TestKnowledgeItemValidatesOfflineAndSeparatesReviewFromHealth(t *testing.T) {
	item := candidateKnowledge()
	item.LegacyReviewStatus = "approved"
	item.LegacyReview = &ReviewRecord{ContentSHA256: HashKnowledgeContent(item.Content), EvidenceSHA256: strings.Repeat("a", 64)}
	item.Review = KnowledgeApproved
	item.Health = KnowledgeStale
	item.Reviewer = "alice"
	item.ContentSHA256 = HashKnowledgeContent(item.Content)
	if err := item.Validate(); err != nil {
		t.Fatalf("approved stale knowledge should validate: %v", err)
	}
	if got := HashKnowledgeContent("other content"); got == item.ContentSHA256 {
		t.Fatal("different content produced the same review hash")
	}
}

func TestKnowledgeSupportsAllKindsReviewStatesAndHealthStates(t *testing.T) {
	kinds := []KnowledgeKind{KnowledgeFact, KnowledgeConvention, KnowledgeBusinessRule, KnowledgeDecision, KnowledgeConstraint}
	reviews := []KnowledgeReviewState{KnowledgeCandidate, KnowledgeApproved, KnowledgeRejected, KnowledgeDeprecated}
	healths := []KnowledgeHealth{KnowledgeVerified, KnowledgeStale, KnowledgeMissing, KnowledgeUnknown}
	for _, kind := range kinds {
		for _, review := range reviews {
			for _, health := range healths {
				item := candidateKnowledge()
				item.Kind, item.Review, item.Health = kind, review, health
				if review != KnowledgeCandidate {
					item.Reviewer = "reviewer"
					item.ContentSHA256 = HashKnowledgeContent(item.Content)
				}
				if err := item.Validate(); err != nil {
					t.Errorf("kind=%s review=%s health=%s: %v", kind, review, health, err)
				}
			}
		}
	}
}

func TestValidateKnowledgeRejectsDuplicateStableIDs(t *testing.T) {
	a, b := candidateKnowledge(), candidateKnowledge()
	b.Content = "different content"
	if err := ValidateKnowledge([]KnowledgeItem{a, b}); err == nil || !strings.Contains(err.Error(), "knowledge[1].id") {
		t.Fatalf("expected duplicate ID diagnostic, got %v", err)
	}
}

func TestKnowledgeValidationRejectsInvalidContracts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*KnowledgeItem)
		want   string
	}{
		{"empty ID", func(k *KnowledgeItem) { k.ID = " " }, "id"},
		{"unknown kind", func(k *KnowledgeItem) { k.Kind = "opinion" }, "kind"},
		{"empty content", func(k *KnowledgeItem) { k.Content = " " }, "content"},
		{"empty origin", func(k *KnowledgeItem) { k.Origin = " " }, "origin"},
		{"unknown review", func(k *KnowledgeItem) { k.Review = "published" }, "review"},
		{"unknown health", func(k *KnowledgeItem) { k.Health = "healthy" }, "health"},
		{"review without reviewer", func(k *KnowledgeItem) { k.Review = KnowledgeApproved }, "reviewer"},
		{"review without content hash", func(k *KnowledgeItem) { k.Review, k.Reviewer = KnowledgeRejected, "alice" }, "content_sha256"},
		{"malformed hash", func(k *KnowledgeItem) { k.ContentSHA256 = "xyz" }, "content_sha256"},
		{"invalid evidence range", func(k *KnowledgeItem) { k.Evidence[0].StartLine, k.Evidence[0].EndLine = 8, 3 }, "evidence[0]"},
		{"malformed evidence hash", func(k *KnowledgeItem) { k.Evidence[0].SHA256 = "1234" }, "evidence[0].sha256"},
		{"blank scope", func(k *KnowledgeItem) { k.Scope.Paths = []string{"  "} }, "scope.paths[0]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := candidateKnowledge()
			tc.mutate(&item)
			if err := item.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q diagnostic, got %v", tc.want, err)
			}
		})
	}
}
