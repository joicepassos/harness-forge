package application

import (
	"context"
	"fmt"
	"harnessforge/internal/symbols/domain"
)

type Parser interface {
	Parse(context.Context, string, []byte) ([]domain.Symbol, error)
}

type Registry interface {
	Resolve(string) (Parser, error)
}

type Extract struct{ parsers Registry }

func NewExtract(parsers Registry) *Extract { return &Extract{parsers: parsers} }

func (e *Extract) Execute(ctx context.Context, language, path string, source []byte) (domain.Report, error) {
	if err := ctx.Err(); err != nil {
		return domain.Report{}, err
	}
	if len(source) > 4<<20 {
		return domain.Report{}, fmt.Errorf("source exceeds 4 MiB")
	}
	parser, err := e.parsers.Resolve(language)
	if err != nil {
		return domain.Report{}, err
	}
	symbols, err := parser.Parse(ctx, path, source)
	if err != nil {
		return domain.Report{}, err
	}
	conventions := make([]domain.Convention, 0, 2)
	for _, kind := range []string{"interface", "implementation"} {
		entry := domain.Convention{Kind: kind}
		for _, symbol := range symbols {
			if symbol.Kind == kind {
				entry.Total++
				if kind == "interface" || len(symbol.Implements) > 0 {
					entry.Matching++
				}
			}
		}
		conventions = append(conventions, entry)
	}
	return domain.Report{Language: language, Symbols: symbols, Conventions: conventions, Limitations: []string{
		"Convention counts are observed numerators and denominators, not probabilities or proof of a rule.",
		"Java uses a bounded declaration parser; Tree-sitter is deferred to avoid a CGO and native distribution dependency.",
	}}, nil
}
