package onboarding

import (
	"context"
	"strings"

	"harnessforge/internal/contextpack"
)

type SetupContextRequest struct {
	Goal         string
	Notes        string
	Model        string
	BudgetTokens int
	UseBM25      bool
	UseMMR       bool
}

// BuildSetupContext reuses the repository context pipeline for onboarding.
// The goal and notes are query material, never repository instructions.
func BuildSetupContext(ctx context.Context, repository string, request SetupContextRequest) (*contextpack.Plan, error) {
	query := strings.Join([]string{
		"project architecture",
		"coding conventions",
		"tests build quality gates",
		"important repository instructions",
		strings.TrimSpace(request.Goal),
		strings.TrimSpace(request.Notes),
	}, "\n")
	return contextpack.Build(ctx, repository, query, request.Model, contextpack.Options{
		BudgetTokens: request.BudgetTokens,
		UseBM25:      request.UseBM25,
		UseMMR:       request.UseMMR,
	})
}
