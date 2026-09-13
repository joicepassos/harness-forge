package application

import (
	"context"
	"harnessforge/internal/symbols/domain"
	"testing"
)

type parser struct{}

func (parser) Parse(context.Context, string, []byte) ([]domain.Symbol, error) {
	return []domain.Symbol{{Kind: "interface"}, {Kind: "implementation", Implements: []string{"Port"}}, {Kind: "implementation"}}, nil
}

type registry struct{}

func (registry) Resolve(string) (Parser, error) { return parser{}, nil }
func TestConventionCountsAreNumeratorsAndDenominators(t *testing.T) {
	report, err := NewExtract(registry{}).Execute(context.Background(), "fixture", "fixture", []byte("source"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Conventions[0].Matching != 1 || report.Conventions[0].Total != 1 || report.Conventions[1].Matching != 1 || report.Conventions[1].Total != 2 {
		t.Fatal(report)
	}
	if len(report.Limitations) == 0 {
		t.Fatal("missing limitations")
	}
}
