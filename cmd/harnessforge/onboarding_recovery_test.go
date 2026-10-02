package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"harnessforge/internal/analyzer"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuidedInitExplainsEvidenceRecoveryAndLocalFallback(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "local fallback", true: "partial recovery"}[partial], func(t *testing.T) {
			root := t.TempDir()
			mod := "module example.com/demo\n\ngo 1.26\n"
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mod), 0644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DEEPSEEK_API_KEY", "never-print-secret")
			proposal := setupAIProposal{Rules: []setupRule{{ID: "invalid", Description: "Rejected rule", Evidence: []setupCitation{{Source: "repository-analysis", Quote: "invented"}}}}}
			if partial {
				proposal.Rules = append(proposal.Rules, setupRule{ID: "valid", Description: "Valid module rule", Evidence: []setupCitation{{Source: "repository-file:go.mod", Quote: "module example.com/demo"}}})
			}
			propose := func(_ context.Context, _ setupProvider, _ *analyzer.Analysis, _ []setupDocument, _ string) (setupAIProposal, error) {
				return recoverSetupProposal(proposal, map[string]string{"repository-file:go.mod": mod})
			}
			answers := []string{"2", "", "", "y", "deepseek", "", "", "", "y"}
			if !partial {
				answers = append(answers, "s")
			}
			answers = append(answers, "3", "s", "")
			var output bytes.Buffer
			if err := runGuidedInit(context.Background(), strings.NewReader(strings.Join(answers, "\n")), &output, root, propose); err != nil {
				t.Fatal(err)
			}
			text := output.String()
			if strings.Contains(text, "never-print-secret") || strings.Contains(text, "invented") {
				t.Fatal("private rejected evidence displayed")
			}
			if partial && !strings.Contains(text, "esses itens foram descartados") {
				t.Fatalf("missing recovery notice: %s", text)
			}
			if !partial && (!strings.Contains(text, "nao citou corretamente um arquivo do projeto") || !strings.Contains(text, "Continuar com uma proposta local")) {
				t.Fatalf("missing explanation/fallback: %s", text)
			}
			data, err := os.ReadFile(filepath.Join(root, ".harness/harness.yaml"))
			if err != nil || bytes.Contains(data, []byte("id: invalid")) || bytes.Contains(data, []byte("invented")) {
				t.Fatalf("unsafe generated proposal: %v", err)
			}
			if partial && !bytes.Contains(data, []byte("id: valid")) {
				t.Fatal("valid rule lost")
			}
		})
	}
}

func TestRecoverSetupProposalKeepsOnlyStrictEvidence(t *testing.T) {
	sources := map[string]string{"repository-file:go.mod": "module example.com/demo\ngo 1.26"}
	valid := setupCitation{Source: "repository-file:go.mod", Quote: "module example.com/demo"}
	for _, invalid := range []setupCitation{
		{Source: "repository-analysis", Quote: valid.Quote},
		{Source: "repository-file:invented.md", Quote: valid.Quote},
		{Source: valid.Source, Quote: "invented document text"},
		{},
	} {
		t.Run(invalid.Source+invalid.Quote, func(t *testing.T) {
			proposal := setupAIProposal{
				Rules:  []setupRule{{ID: "valid", Description: "Valid rule", Evidence: []setupCitation{valid}}, {ID: "invalid", Description: "Invalid rule", Evidence: []setupCitation{valid, invalid}}},
				Skills: []setupSkill{{ID: "skill", Description: "Invalid skill", Steps: []string{"Check"}, Evidence: []setupCitation{invalid}}},
			}
			got, err := recoverSetupProposal(proposal, sources)
			if err != nil || len(got.Rules) != 1 || got.Rules[0].ID != "valid" || len(got.Skills) != 0 || got.DiscardedRules != 1 || got.DiscardedSkills != 1 {
				t.Fatalf("unexpected recovery: %+v, %v", got, err)
			}
			if err := validateSetupProposal(got, sources); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(got)
			if err != nil || strings.Contains(string(encoded), "Discarded") {
				t.Fatalf("recovery metadata leaked: %s, %v", encoded, err)
			}
		})
	}
}

func TestRecoverSetupProposalAllInvalidRejectsSafely(t *testing.T) {
	proposal := setupAIProposal{Rules: []setupRule{{ID: "secret-rule", Description: "Rule", Evidence: []setupCitation{{Source: "secret-document", Quote: "secret text"}}}}}
	_, err := recoverSetupProposal(proposal, nil)
	var evidenceErr *setupEvidenceError
	if !errors.As(err, &evidenceErr) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected safe evidence error, got %v", err)
	}
}

func TestRecoverSetupProposalCanKeepValidSkill(t *testing.T) {
	sources := map[string]string{"repository-file:README.md": "Run go test ./..."}
	valid := setupCitation{Source: "repository-file:README.md", Quote: "go test ./..."}
	proposal := setupAIProposal{
		Rules:  []setupRule{{ID: "rule", Description: "Missing evidence"}},
		Skills: []setupSkill{{ID: "test", Description: "Test", Steps: []string{"Run tests"}, Evidence: []setupCitation{valid}}},
	}
	got, err := recoverSetupProposal(proposal, sources)
	if err != nil || len(got.Rules) != 0 || len(got.Skills) != 1 || got.DiscardedRules != 1 {
		t.Fatalf("expected valid skill recovery: %+v, %v", got, err)
	}
}

func TestRecoverSetupProposalDoesNotHideStructuralErrors(t *testing.T) {
	invalid := setupRule{ID: "duplicate", Description: "Rule"}
	for name, proposal := range map[string]setupAIProposal{
		"duplicate":         {Rules: []setupRule{invalid, invalid}},
		"invalid ID":        {Rules: []setupRule{{ID: "SECRET_VALUE", Description: "Rule"}}},
		"empty description": {Rules: []setupRule{{ID: "rule"}}},
		"invalid step":      {Skills: []setupSkill{{ID: "skill", Description: "Skill", Steps: []string{""}}}},
		"architecture":      {Architecture: []string{""}, Rules: []setupRule{invalid}},
		"too many rules":    {Rules: make([]setupRule, 9)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := recoverSetupProposal(proposal, nil)
			var evidenceErr *setupEvidenceError
			if err == nil || errors.As(err, &evidenceErr) || strings.Contains(err.Error(), "SECRET_VALUE") {
				t.Fatalf("expected structural rejection, got %v", err)
			}
		})
	}
}

func TestRecoverSetupProposalEmptyEvidenceFreeProposal(t *testing.T) {
	got, err := recoverSetupProposal(setupAIProposal{Summary: "Summary"}, nil)
	if err != nil || got.Summary != "Summary" {
		t.Fatalf("empty arrays should stay valid: %+v, %v", got, err)
	}
}
