package application

import (
	"context"
	"harnessforge/internal/llm/domain"
	"testing"
)

type structuredProvider struct {
	content string
	request domain.Request
}

func (p *structuredProvider) Generate(_ context.Context, r domain.Request) (*domain.Response, error) {
	p.request = r
	return &domain.Response{Content: p.content}, nil
}
func TestStructuredOutputRejectsInvalidSchemaAndCitations(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		valid         bool
	}{
		{"valid", `{"architecture":["ddd"],"patterns":[{"name":"ddd","confidence":0.7,"evidence":[{"source":"prompt","quote":"domain layer"}]}]}`, true},
		{"abstention", `{"architecture":[],"patterns":[]}`, true},
		{"plain text", "This is DDD", false},
		{"missing fields", `{"architecture":[]}`, false},
		{"null", `{"architecture":[],"patterns":null}`, false},
		{"missing confidence", `{"architecture":[],"patterns":[{"name":"ddd","evidence":[{"source":"prompt","quote":"domain layer"}]}]}`, false},
		{"fabricated citation", `{"architecture":[],"patterns":[{"name":"ddd","confidence":0.7,"evidence":[{"source":"prompt","quote":"not supplied"}]}]}`, false},
		{"unsupported architecture", `{"architecture":["ddd"],"patterns":[]}`, false},
		{"invalid confidence", `{"architecture":[],"patterns":[{"name":"ddd","confidence":2,"evidence":[{"source":"prompt","quote":"domain layer"}]}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &structuredProvider{content: tc.content}
			resolver := &fakeResolver{strategy: provider}
			_, err := NewAsk(resolver).Structured(context.Background(), "deepseek", "", "There is a domain layer", "")
			if (err == nil) != tc.valid {
				t.Fatalf("valid %v error %v", tc.valid, err)
			}
			if !provider.request.JSON {
				t.Fatal("JSON not requested")
			}
		})
	}
}
