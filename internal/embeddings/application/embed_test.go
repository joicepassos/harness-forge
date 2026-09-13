package application

import (
	"context"
	"harnessforge/internal/embeddings/domain"
	"testing"
)

type provider struct {
	vector domain.Vector
	called bool
}

func (p *provider) Embed(context.Context, string, string) (domain.Vector, error) {
	p.called = true
	return p.vector, nil
}
func TestEmbedValidatesInputAndResponse(t *testing.T) {
	p := &provider{vector: domain.Vector{Model: "fixture-v1", Dimensions: 2, Values: []float64{1, 2}}}
	result, err := NewEmbed(p).Execute(context.Background(), "fixture-v1", "related text")
	if err != nil || result.Dimensions != 2 {
		t.Fatal(result, err)
	}
	p.called = false
	if _, err := NewEmbed(p).Execute(context.Background(), "fixture-v1", " "); err == nil || p.called {
		t.Fatal("empty input reached provider")
	}
	p.vector.Dimensions = 3
	if _, err := NewEmbed(p).Execute(context.Background(), "fixture-v1", "text"); err == nil {
		t.Fatal("malformed response accepted")
	}
}
