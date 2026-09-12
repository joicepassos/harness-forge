package contextpack

import (
	"context"
	"harnessforge/internal/contextpack/application"
	"harnessforge/internal/contextpack/domain"
	"harnessforge/internal/contextpack/infrastructure"
)

const DefaultBudgetTokens = domain.DefaultBudgetTokens

type Options = domain.Options
type Plan = domain.Plan
type Excerpt = domain.Excerpt
type Comparison = domain.Comparison

func Build(ctx context.Context, repositoryPath, prompt, model string, options Options) (*Plan, error) {
	return infrastructure.Build(ctx, repositoryPath, prompt, model, options)
}

func EncodePrompt(prompt string, plan *Plan) (string, map[string]string, error) {
	return application.EncodePrompt(prompt, plan)
}

func Sources(prompt string, plan *Plan) map[string]string {
	return application.Sources(prompt, plan)
}
