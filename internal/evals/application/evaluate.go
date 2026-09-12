package application

import (
	"fmt"
	"harnessforge/internal/evals/domain"
	"strings"
)

func Evaluate(dataset domain.Dataset, results domain.Results) (domain.Report, error) {
	if dataset.Version != 1 || dataset.DatasetVersion == "" || len(dataset.Cases) == 0 {
		return domain.Report{}, fmt.Errorf("invalid evaluation dataset")
	}
	seenCases := map[string]bool{}
	for _, testCase := range dataset.Cases {
		if testCase.ID == "" || seenCases[testCase.ID] || len(testCase.RelevantSources) == 0 {
			return domain.Report{}, fmt.Errorf("invalid or duplicate dataset case")
		}
		seenCases[testCase.ID] = true
	}
	if results.IndexVersion == "" || results.Model == "" || results.PromptVersion == "" || results.RubricVersion == "" {
		return domain.Report{}, fmt.Errorf("results must record index, model, prompt and rubric versions")
	}
	byID := map[string]domain.CaseResult{}
	for _, result := range results.Cases {
		if result.Tokens < 0 || result.LatencyMS < 0 {
			return domain.Report{}, fmt.Errorf("negative result metric")
		}
		if result.ID == "" || byID[result.ID].ID != "" {
			return domain.Report{}, fmt.Errorf("invalid or duplicate result case id")
		}
		byID[result.ID] = result
	}
	report := domain.Report{DatasetVersion: dataset.DatasetVersion, IndexVersion: results.IndexVersion, Model: results.Model, PromptVersion: results.PromptVersion, RubricVersion: results.RubricVersion, Limitations: []string{"Retrieval metrics use exact source IDs.", "Correctness uses required-term matching.", "Faithfulness checks only citations against retrieved relevant sources; human review is required.", "No LLM judge is used."}}
	for _, testCase := range dataset.Cases {
		result, ok := byID[testCase.ID]
		caseReport := domain.CaseReport{ID: testCase.ID}
		if !ok {
			caseReport.Error = "missing result"
		} else {
			caseReport = score(testCase, result)
		}
		report.Cases = append(report.Cases, caseReport)
	}
	report.Aggregate = aggregate(report.Cases)
	return report, nil
}

func score(testCase domain.Case, result domain.CaseResult) domain.CaseReport {
	relevant := set(testCase.RelevantSources)
	retrieved := set(result.RetrievedSources)
	hits := 0
	for source := range relevant {
		if retrieved[source] {
			hits++
		}
	}
	recall, precision := 0.0, 0.0
	if len(relevant) > 0 {
		recall = float64(hits) / float64(len(relevant))
	}
	if len(retrieved) > 0 {
		precision = float64(hits) / float64(len(retrieved))
	}
	correct := result.Error == ""
	answer := strings.ToLower(result.Answer)
	for _, term := range testCase.RequiredTerms {
		if !strings.Contains(answer, strings.ToLower(term)) {
			correct = false
		}
	}
	faithful := result.Error == "" && len(result.CitedSources) > 0
	for _, source := range result.CitedSources {
		if !retrieved[source] || !relevant[source] {
			faithful = false
		}
	}
	return domain.CaseReport{ID: testCase.ID, RecallAtK: recall, PrecisionAtK: precision, Correctness: correct, Faithfulness: faithful, Tokens: result.Tokens, LatencyMS: result.LatencyMS, Error: result.Error}
}

func aggregate(cases []domain.CaseReport) domain.Aggregate {
	result := domain.Aggregate{}
	if len(cases) == 0 {
		return result
	}
	for _, item := range cases {
		result.RecallAtK += item.RecallAtK
		result.PrecisionAtK += item.PrecisionAtK
		if item.Correctness {
			result.Correctness++
		}
		if item.Faithfulness {
			result.Faithfulness++
		}
		result.Tokens += item.Tokens
		result.LatencyMS += item.LatencyMS
		if item.Error != "" {
			result.Failures++
		}
	}
	count := float64(len(cases))
	result.RecallAtK /= count
	result.PrecisionAtK /= count
	result.Correctness /= count
	result.Faithfulness /= count
	return result
}

func Compare(baseline, candidate domain.Report) (domain.Comparison, error) {
	if baseline.DatasetVersion == "" || baseline.DatasetVersion != candidate.DatasetVersion {
		return domain.Comparison{}, fmt.Errorf("reports must use the same dataset version")
	}
	b, c := baseline.Aggregate, candidate.Aggregate
	return domain.Comparison{Baseline: b, Candidate: c, Delta: domain.Aggregate{RecallAtK: c.RecallAtK - b.RecallAtK, PrecisionAtK: c.PrecisionAtK - b.PrecisionAtK, Correctness: c.Correctness - b.Correctness, Faithfulness: c.Faithfulness - b.Faithfulness, Tokens: c.Tokens - b.Tokens, LatencyMS: c.LatencyMS - b.LatencyMS, Failures: c.Failures - b.Failures}}, nil
}

func set(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}
