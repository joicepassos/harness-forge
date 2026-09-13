package application

import (
	"context"
	"harnessforge/internal/drift/domain"
	harnessdomain "harnessforge/internal/harness/domain"
	"testing"
)

type harnessLoader struct{ harness harnessdomain.Harness }

func (l harnessLoader) Load(string) (harnessdomain.Harness, error) { return l.harness, nil }

type reader struct {
	found bool
	err   error
}

func (r reader) Contains(context.Context, string, string) (bool, error) { return r.found, r.err }

func TestDetectShowsAlignedDifferenceAndUnsupportedRule(t *testing.T) {
	h := harnessdomain.Harness{Rules: []harnessdomain.Rule{
		{ID: "aligned", Status: "approved", Evidence: []harnessdomain.Evidence{{File: "a.go", Symbol: "Present"}}},
		{ID: "unsupported", Status: "approved"},
	}}
	report, err := NewDetect(harnessLoader{h}, reader{found: true}).Execute(context.Background(), "harness.yaml")
	if err != nil || report.Occurrences[0].Status != domain.StatusAligned || report.Occurrences[1].Status != domain.StatusNotEvaluated {
		t.Fatalf("%#v %v", report, err)
	}
	report, err = NewDetect(harnessLoader{harnessdomain.Harness{Rules: []harnessdomain.Rule{{ID: "changed", Status: "approved", Evidence: []harnessdomain.Evidence{{File: "a.go", Symbol: "Missing"}}}}}}, reader{}).Execute(context.Background(), "harness.yaml")
	if err != nil || report.Occurrences[0].Status != domain.StatusDifference || report.Occurrences[0].CodeProposal == "" || report.Occurrences[0].HarnessProposal == "" {
		t.Fatalf("%#v %v", report, err)
	}
}
