package application

import (
	"context"
	"errors"
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

type versionReader struct {
	reader
	baseline bool
}

func (r versionReader) ContainsAt(context.Context, string, string, string) (bool, error) {
	return r.baseline, nil
}

func TestEveryEvidenceAndHistoricalBaselineIsEvaluated(t *testing.T) {
	h := harnessdomain.Harness{Rules: []harnessdomain.Rule{{ID: "rule", Status: "approved", Evidence: []harnessdomain.Evidence{{File: "unsupported"}, {File: "code.go", Symbol: "Old", Revision: "HEAD"}}}}}
	report, err := NewDetect(harnessLoader{h}, versionReader{baseline: true}).Execute(context.Background(), "harness.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Occurrences) != 2 {
		t.Fatal(report)
	}
	change := report.Occurrences[1]
	if change.Status != domain.StatusDifference || change.BaselineStatus != "verified" || change.Revision != "HEAD" {
		t.Fatal(change)
	}
	if len(change.Explanations) != 4 {
		t.Fatal("possible intentional change and violation must both be explained")
	}
	report, err = NewDetect(harnessLoader{h}, versionReader{}).Execute(context.Background(), "harness.yaml")
	if err != nil || report.Occurrences[1].Status != domain.StatusNotEvaluated || report.Occurrences[1].BaselineStatus != "invalid" {
		t.Fatalf("%v %v", report, err)
	}
}

func TestReaderFailureIsNotClassifiedAsDriftAndCancellationPropagates(t *testing.T) {
	h := harnessdomain.Harness{Rules: []harnessdomain.Rule{{ID: "rule", Status: "approved", Evidence: []harnessdomain.Evidence{{File: "code.go", Symbol: "Old"}}}}}
	report, err := NewDetect(harnessLoader{h}, reader{err: errors.New("access denied")}).Execute(context.Background(), "harness.yaml")
	if err != nil || report.Occurrences[0].Status != domain.StatusNotEvaluated {
		t.Fatalf("%v %v", report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewDetect(harnessLoader{h}, reader{}).Execute(ctx, "harness.yaml"); err != context.Canceled {
		t.Fatal(err)
	}
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
