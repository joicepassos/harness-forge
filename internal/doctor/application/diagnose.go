package application

import (
	"context"
	"fmt"
	"harnessforge/internal/doctor/domain"
)

type Source interface {
	Diagnose(context.Context, string, string) ([]domain.Diagnostic, error)
}

type Diagnose struct{ source Source }

func NewDiagnose(source Source) *Diagnose { return &Diagnose{source: source} }

func (d *Diagnose) Execute(ctx context.Context, harnessPath, repository string) (domain.Report, error) {
	report := domain.Report{Limitations: []string{
		"Diagnostics inspect declared Harness IR and local file references; they do not prove that a rule is correct or complete.",
		"Duplicate detection compares normalized instruction text and can flag intentionally similar instructions for human review.",
		"Context size warnings use a documented byte threshold, not a model tokenizer or answer-quality score.",
	}}
	diagnostics, err := d.source.Diagnose(ctx, harnessPath, repository)
	if err != nil {
		return report, fmt.Errorf("diagnose harness: %w", err)
	}
	report.Diagnostics = diagnostics
	report.Sort()
	return report, nil
}
