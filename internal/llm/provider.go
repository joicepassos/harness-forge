package llm

import (
	"harnessforge/internal/llm/application"
	"harnessforge/internal/llm/infrastructure/chatcompat"
	"os"
)

// NewAsk composes the application with environment-backed infrastructure.
func NewAsk() *application.Ask {
	return application.NewAsk(chatcompat.NewRegistry(os.Getenv))
}

func DefaultModel(provider string) string {
	return chatcompat.NewRegistry(os.Getenv).DefaultModel(provider)
}
