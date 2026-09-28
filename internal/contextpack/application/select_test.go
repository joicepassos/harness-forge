package application

import (
	"context"
	"encoding/json"
	"errors"
	"harnessforge/internal/contextpack/domain"
	"strings"
	"testing"
)

type testTokenCounter struct {
	model string
	err   error
	value int
	force bool
}

func (c *testTokenCounter) Name() string { return "test-counter-v1" }
func (c *testTokenCounter) Count(_ context.Context, model string, payload []byte) (int, error) {
	c.model = model
	if c.err != nil {
		return 0, c.err
	}
	if c.force {
		return c.value, nil
	}
	return (len(payload) + 3) / 4, nil
}

func TestSelectWithBudgetDeclaresCounterErrorFallback(t *testing.T) {
	counter := &testTokenCounter{err: errors.New("counter unavailable")}
	plan := SelectWithBudget(context.Background(), []domain.Excerpt{{
		ID: "a", Source: "a", Text: "authentication", Relevance: 1,
	}}, "authentication", Budget{MaxInputTokens: 500, Counter: counter}, domain.Metrics{})
	if want := "test-counter-v1; fallback=payload-byte-upper-bound-v1 (counter error)"; plan.Estimator != want {
		t.Fatalf("estimator = %q, want %q", plan.Estimator, want)
	}
}

func TestSelectWithBudgetDeclaresNegativeCounterFallback(t *testing.T) {
	counter := &testTokenCounter{force: true, value: -1}
	plan := SelectWithBudget(context.Background(), []domain.Excerpt{{
		ID: "a", Source: "a", Text: "authentication", Relevance: 1,
	}}, "authentication", Budget{MaxInputTokens: 500, Counter: counter}, domain.Metrics{})
	if want := "test-counter-v1; fallback=payload-byte-upper-bound-v1 (counter returned negative value)"; plan.Estimator != want {
		t.Fatalf("estimator = %q, want %q", plan.Estimator, want)
	}
}

func TestConservativeByteEstimatorDeclaresItsApproximation(t *testing.T) {
	estimator := ConservativeByteEstimator{}
	if estimator.Name() != "payload-byte-upper-bound-v1" {
		t.Fatalf("name = %q", estimator.Name())
	}
	count, err := estimator.Count(context.Background(), "model", []byte("café"))
	if err != nil || count != len([]byte("café")) {
		t.Fatalf("count = %d, err = %v", count, err)
	}
}

func TestSelectWithBudgetUsesProvidedCounterAndReserve(t *testing.T) {
	counter := &testTokenCounter{}
	plan := SelectWithBudget(context.Background(), []domain.Excerpt{{
		ID: "a", Source: "repository-file:a.go", Path: "a.go", Text: "authentication validation", Relevance: 1,
	}}, "authentication", Budget{MaxInputTokens: 300, ReserveTokens: 17, Model: "provider-model", Counter: counter}, domain.Metrics{})
	if plan.Estimator != "test-counter-v1" {
		t.Fatalf("estimator = %q", plan.Estimator)
	}
	if plan.PayloadReserveTokens != 17 {
		t.Fatalf("reserve = %d", plan.PayloadReserveTokens)
	}
	if counter.model != "provider-model" {
		t.Fatalf("model = %q", counter.model)
	}
	if plan.EstimatedTokens > plan.BudgetTokens {
		t.Fatalf("budget exceeded: %d > %d", plan.EstimatedTokens, plan.BudgetTokens)
	}
}

func TestSelectWithBudgetCanUseMMRLikeDiversity(t *testing.T) {
	plan := SelectWithBudget(context.Background(), []domain.Excerpt{
		{ID: "auth-1", Source: "a", Text: "authentication validation token", Relevance: 100},
		{ID: "auth-2", Source: "b", Text: "authentication validation token", Relevance: 99},
		{ID: "tests", Source: "c", Text: "authentication integration tests", Relevance: 80},
	}, "authentication", Budget{MaxInputTokens: 900, UseMMR: true}, domain.Metrics{})
	if len(plan.Included) < 2 || plan.Included[0].ID != "auth-1" || plan.Included[1].ID != "tests" {
		t.Fatalf("MMR order = %+v", plan.Included)
	}
}

func FuzzSelectNeverExceedsBudget(f *testing.F) {
	f.Add("auth service", "internal/auth/service.go", "Validate token")
	f.Add("café", "src/évidence.go", "unicode content")
	f.Fuzz(func(t *testing.T, prompt, path, content string) {
		if len(content) > 64<<10 {
			t.Skip()
		}
		if RequiredTokens(prompt) > 1200 {
			t.Skip()
		}
		plan := Select([]domain.Excerpt{{
			ID: "fuzz", Source: "repository-file:" + path, Path: path,
			Text: content, EstimatedTokens: EstimateTokens(content),
		}}, prompt, 1200, domain.Metrics{})
		if plan.EstimatedTokens > plan.BudgetTokens {
			t.Fatalf("budget violated: used=%d budget=%d", plan.EstimatedTokens, plan.BudgetTokens)
		}
	})
}

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
