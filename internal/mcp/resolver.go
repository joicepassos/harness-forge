// Package mcp exposes a read-only MCP server for Forge context resources.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"harnessforge/internal/contextpack"
)

const ProtocolVersion = "2025-06-18"

const DefaultBudgetTokens = contextpack.DefaultBudgetTokens

type Resolver struct{}

// ResolveContext produces a bounded, provenance-bearing context plan. It reads
// project files and approved Forge knowledge only; it never writes or executes.
func (Resolver) ResolveContext(ctx context.Context, repository, prompt, model string, budget int, taskPaths ...string) (*contextpack.Plan, error) {
	if budget < 1 {
		return nil, fmt.Errorf("context budget must be positive")
	}
	return contextpack.Build(ctx, repository, prompt, model, contextpack.Options{BudgetTokens: budget, Layout: "forge", TaskPaths: taskPaths})
}

// ReadResource returns a bounded context plan serialized as JSON. Clients
// decide whether to fetch resources; this package does not push context.
func (r Resolver) ReadResource(ctx context.Context, repository, prompt, model string, budget int, taskPaths ...string) ([]byte, error) {
	plan, err := r.ResolveContext(ctx, repository, prompt, model, budget, taskPaths...)
	if err != nil {
		return nil, err
	}
	return json.Marshal(plan)
}
