package application

import (
	"context"
	"fmt"
	"harnessforge/internal/embeddings/domain"
	"strings"
)

type Provider interface {
	Embed(context.Context, string, string) (domain.Vector, error)
}
type Embed struct{ provider Provider }

func NewEmbed(provider Provider) *Embed { return &Embed{provider: provider} }
func (e *Embed) Execute(ctx context.Context, model, input string) (domain.Vector, error) {
	if err := ctx.Err(); err != nil {
		return domain.Vector{}, err
	}
	if strings.TrimSpace(input) == "" {
		return domain.Vector{}, fmt.Errorf("embedding input must not be empty")
	}
	if len([]byte(input)) > 32<<10 {
		return domain.Vector{}, fmt.Errorf("embedding input exceeds 32 KiB")
	}
	vector, err := e.provider.Embed(ctx, model, input)
	if err != nil {
		return domain.Vector{}, err
	}
	if err := vector.Validate(); err != nil {
		return domain.Vector{}, fmt.Errorf("invalid embedding response: %w", err)
	}
	return vector, nil
}
