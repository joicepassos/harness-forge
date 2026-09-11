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
