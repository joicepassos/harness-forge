package main

import (
	"context"

	"harnessforge/internal/llm/infrastructure/chatcompat"
)

func withSetupCallReporter(ctx context.Context, report func(chatcompat.CallEvent)) context.Context {
	return chatcompat.WithCallObserver(ctx, report)
}
