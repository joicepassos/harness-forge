package application

import (
	"encoding/json"
	"harnessforge/internal/contextpack/domain"
	"strings"
	"testing"
)

func TestSelectBudgetUsesSerializedSourcesWithEscapedText(t *testing.T) {
	prompt := "Explain webhook \"auth\" café"
	candidates := []domain.Excerpt{
		{
			ID:              "a",
			Source:          "repository-file:a.go",
			Path:            "a.go",
			Text:            "webhook \"auth\" café",
			Relevance:       100,
			EstimatedTokens: EstimateTokens("webhook \"auth\" café"),
			Origins:         []string{"a.go"},
		},
		{
			ID:              "b",
			Source:          "repository-file:b.go",
			Path:            "b.go",
			Text:            "webhook " + strings.Repeat("padding ", 100),
			Relevance:       90,
			EstimatedTokens: 200,
			Origins:         []string{"b.go"},
		},
	}

	plan := Select(candidates, prompt, 220, domain.Metrics{PreviousAnalyzerJSONEstimatedTokens: 1000, UnfilteredCandidateEstimatedTokens: 1200})
	if plan.EstimatedTokens > plan.BudgetTokens {
		t.Fatalf("estimated tokens %d exceed budget %d", plan.EstimatedTokens, plan.BudgetTokens)
	}
	sources := Sources(prompt, plan)
	data, err := json.Marshal(sources)
	if err != nil {
		t.Fatal(err)
	}
	actual := framingTokens + EstimateTokens(string(data))
	if plan.EstimatedTokens != actual {
		t.Fatalf("estimated tokens = %d, want serialized estimate %d", plan.EstimatedTokens, actual)
	}
}
