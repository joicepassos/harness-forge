package domain

import "context"

type Provider interface {
	Generate(ctx context.Context, request Request) (*Response, error)
}

type Request struct {
	JSON         bool
	SystemPrompt string
	Prompt       string
	Temperature  float64
}

type Response struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

type StreamingProvider interface {
	Stream(context.Context, Request, func(string) error) error
}
