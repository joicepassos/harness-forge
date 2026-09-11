package application

import (
	"context"
	"errors"
	"harnessforge/internal/llm/domain"
	"testing"
)

type fakeStrategy struct {
	got domain.Request
	ctx context.Context
}

func (f *fakeStrategy) Generate(ctx context.Context, r domain.Request) (*domain.Response, error) {
	f.got = r
	f.ctx = ctx
	return &domain.Response{Content: "ok"}, nil
}

type fakeResolver struct {
	strategy    domain.Provider
	name, model string
	err         error
}

func (f *fakeResolver) Resolve(name, model string) (domain.Provider, error) {
	f.name = name
	f.model = model
	return f.strategy, f.err
}

func TestAskUsesSelectedStrategy(t *testing.T) {
	strategy := &fakeStrategy{}
	resolver := &fakeResolver{strategy: strategy}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := NewAsk(resolver).Execute(ctx, "gemini", "chosen", "summarize")
	if err != nil || result.Content != "ok" {
		t.Fatalf("unexpected result %v %v", result, err)
	}
	if resolver.name != "gemini" || resolver.model != "chosen" || strategy.got.Prompt != "summarize" || strategy.ctx != ctx {
		t.Fatal("strategy selection or request propagation failed")
	}
}
func TestAskRejectsEmptyPromptAndPropagatesSelectionError(t *testing.T) {
	expected := errors.New("unavailable")
	resolver := &fakeResolver{err: expected}
	ask := NewAsk(resolver)
	if _, err := ask.Execute(context.Background(), "provider", "model", " "); err == nil {
		t.Fatal("empty prompt accepted")
	}
	if resolver.name != "" {
		t.Fatal("resolved provider for empty prompt")
	}
	if _, err := ask.Execute(context.Background(), "provider", "model", "hello"); !errors.Is(err, expected) {
		t.Fatalf("error = %v", err)
	}
}
