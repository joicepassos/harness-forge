package application

import (
	"context"
	"harnessforge/internal/contextpack/domain"
	"strings"
	"testing"
)

func TestSelectPrefersKnowledgeForMatchingTaskScope(t *testing.T) {
	plan := SelectWithBudget(context.Background(), []domain.Excerpt{
		{ID: "backend", Source: "forge-knowledge:backend", Path: ".forge/knowledge/backend.md", Text: "UI validation rules", KnowledgeID: "backend", KnowledgeScope: []string{"services/backend/**"}},
		{ID: "frontend", Source: "forge-knowledge:frontend", Path: ".forge/knowledge/frontend.md", Text: "UI validation rules", KnowledgeID: "frontend", KnowledgeScope: []string{"apps/frontend/**"}},
	}, "frontend form", Budget{MaxInputTokens: 3000}, domain.Metrics{})

	if len(plan.Included) != 1 || plan.Included[0].KnowledgeID != "frontend" {
		t.Fatalf("included = %+v; want only frontend-scoped knowledge", plan.Included)
	}
	if len(plan.Excluded) != 1 || plan.Excluded[0].KnowledgeID != "backend" || !strings.Contains(plan.Excluded[0].Reason, "scope") {
		t.Fatalf("excluded = %+v; want backend scope explanation", plan.Excluded)
	}
}

func TestSelectLeavesKnowledgeScopesEligibleWhenPromptNamesNoScope(t *testing.T) {
	plan := SelectWithBudget(context.Background(), []domain.Excerpt{
		{ID: "backend", Source: "forge-knowledge:backend", Text: "shared validation approach", KnowledgeID: "backend", KnowledgeScope: []string{"services/backend/**"}},
		{ID: "frontend", Source: "forge-knowledge:frontend", Text: "shared validation approach", KnowledgeID: "frontend", KnowledgeScope: []string{"apps/frontend/**"}},
	}, "shared validation", Budget{MaxInputTokens: 3000}, domain.Metrics{})
	if len(plan.Included) != 1 || len(plan.Excluded) != 1 {
		t.Fatalf("expected duplicate content to deduplicate after scope-neutral selection: included=%+v excluded=%+v", plan.Included, plan.Excluded)
	}
	if strings.Contains(plan.Excluded[0].Reason, "scope") {
		t.Fatalf("scope was guessed despite no matching scope term: %+v", plan.Excluded[0])
	}
}

func TestSelectReportsKnowledgeBudgetOverflowWithoutTruncating(t *testing.T) {
	text := "authentication " + strings.Repeat("approved detail ", 100)
	plan := SelectWithBudget(context.Background(), []domain.Excerpt{{
		ID: "approved", Source: "forge-knowledge:approved", Text: text, KnowledgeID: "approved",
	}}, "authentication", Budget{MaxInputTokens: 200}, domain.Metrics{})

	if !plan.BudgetOverflow || len(plan.OverflowExcerptIDs) != 1 || plan.OverflowExcerptIDs[0] != "approved" || plan.OverflowTokens <= 0 {
		t.Fatalf("overflow outcome = %+v", plan)
	}
	if len(plan.Included) != 0 || len(plan.Excluded) != 1 {
		t.Fatalf("oversized approved knowledge was partially included: %+v", plan)
	}
	if plan.Excluded[0].Text != text {
		t.Fatalf("approved knowledge text was modified: got %d bytes, want %d", len(plan.Excluded[0].Text), len(text))
	}
}

func TestSelectMarksOrdinaryExcerptCompression(t *testing.T) {
	text := "authentication " + strings.Repeat("implementation detail ", 50)
	plan := SelectWithBudget(context.Background(), []domain.Excerpt{{
		ID: "source", Source: "repository-file:internal/auth.go", Path: "internal/auth.go", Text: text,
	}}, "authentication", Budget{MaxInputTokens: 3000}, domain.Metrics{})
	if len(plan.Included) != 1 {
		t.Fatalf("included = %+v", plan.Included)
	}
	if plan.Included[0].CompressedFrom != "source" || !strings.Contains(plan.Included[0].Reason, "compressed") {
		t.Fatalf("compression was not disclosed: %+v", plan.Included[0])
	}
}
