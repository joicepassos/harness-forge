package application

import (
	"context"
	"harnessforge/internal/github/domain"
	"testing"
)

type discussionFixture []domain.Discussion

func (d discussionFixture) Read(context.Context, string) ([]domain.Discussion, error) { return d, nil }

func TestRecurringOpinionsRemainReviewableOpinions(t *testing.T) {
	d := discussionFixture{{Kind: "comment", Body: "Prefer adapters", URL: "https://example.test/1"}, {Kind: "comment", Body: "prefer   adapters", URL: "https://example.test/2"}, {Kind: "comment", Body: "Decision: use ports", URL: "https://example.test/3"}}
	report, err := NewExtract(d).Execute(context.Background(), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Candidates) != 2 || len(report.Candidates[0].Sources) != 2 || report.Candidates[0].Classification != "discussion" {
		t.Fatal(report)
	}
	for _, candidate := range report.Candidates {
		if !candidate.RequiresHumanReview {
			t.Fatal("approval was inferred")
		}
	}
}
