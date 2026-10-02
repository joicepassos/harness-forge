package infrastructure

import (
	"context"
	"strings"
	"testing"

	"harnessforge/internal/contextpack/domain"
)

type promptOverflowCounter struct{ model string }

func (c *promptOverflowCounter) Name() string { return "prompt-overflow-test-counter" }
func (c *promptOverflowCounter) Count(_ context.Context, model string, payload []byte) (int, error) {
	c.model = model
	return len(payload) / 4, nil
}

func TestBuildReportsPromptEnvelopeExclusionWithConfiguredEstimator(t *testing.T) {
	root := t.TempDir()
	counter := &promptOverflowCounter{}
	plan, err := Build(context.Background(), root, strings.Repeat("webhook authentication ", 4), "pilot-model", domain.Options{BudgetTokens: 10, Counter: counter})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.BudgetOverflow || plan.OverflowTokens <= 0 || plan.Estimator != counter.Name() || counter.model != "pilot-model" {
		t.Fatalf("prompt overflow accounting mismatch: plan=%+v counter=%+v", plan, counter)
	}
	if len(plan.Excluded) != 1 || plan.Excluded[0].ID != "prompt-envelope" || plan.Excluded[0].Reason == "" {
		t.Fatalf("prompt overflow lacks an explicit exclusion: %+v", plan)
	}
}
