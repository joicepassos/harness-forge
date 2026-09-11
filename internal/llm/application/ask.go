package application

import (
	"context"
	"fmt"
	"harnessforge/internal/llm/domain"
	"strings"
)

// ProviderResolver is a port implemented by the configured provider registry.
type ProviderResolver interface {
	Resolve(name, model string) (domain.Provider, error)
}

type Ask struct{ providers ProviderResolver }

func NewAsk(providers ProviderResolver) *Ask { return &Ask{providers: providers} }

func (ask *Ask) Execute(ctx context.Context, name, model, prompt string) (*domain.Response, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt must not be empty")
	}
	provider, err := ask.providers.Resolve(name, model)
	if err != nil {
		return nil, err
	}
	return provider.Generate(ctx, domain.Request{
		SystemPrompt: "You are HarnessForge, a concise assistant for repository analysis.",
		Prompt:       prompt, Temperature: 0.2,
	})
}
