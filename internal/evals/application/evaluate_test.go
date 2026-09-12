package application

import (
	"harnessforge/internal/evals/domain"
	"testing"
)

func TestEvaluateReportsMetricsAndFailures(t *testing.T) {
	dataset := domain.Dataset{Version: 1, DatasetVersion: "v1", Cases: []domain.Case{{ID: "one", RelevantSources: []string{"a", "b"}, RequiredTerms: []string{"controller"}}, {ID: "two", RelevantSources: []string{"c"}, RequiredTerms: []string{"service"}}}}
	results := domain.Results{IndexVersion: "index-v1", Model: "test", PromptVersion: "prompt-v1", RubricVersion: "human-v1", Cases: []domain.CaseResult{{ID: "one", RetrievedSources: []string{"a", "noise"}, CitedSources: []string{"a"}, Answer: "controller", Tokens: 9, LatencyMS: 2}, {ID: "two", Error: "timeout"}}}
	report, err := Evaluate(dataset, results)
	if err != nil {
		t.Fatal(err)
	}
	if report.Aggregate.Failures != 1 || report.Cases[0].RecallAtK != 0.5 || report.Cases[0].PrecisionAtK != 0.5 || !report.Cases[0].Faithfulness {
		t.Fatalf("unexpected report: %+v", report)
	}
	if _, err := Compare(report, domain.Report{DatasetVersion: "other"}); err == nil {
		t.Fatal("mixed dataset versions compared")
	}
	_, err = Evaluate(domain.Dataset{Version: 1, DatasetVersion: "v1", Cases: []domain.Case{{ID: "one", RelevantSources: []string{"a"}}, {ID: "one", RelevantSources: []string{"b"}}}}, results)
	if err == nil {
		t.Fatal("duplicate dataset IDs accepted")
	}
}
