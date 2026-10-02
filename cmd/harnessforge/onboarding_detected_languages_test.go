package main

import (
	"bufio"
	"bytes"
	"harnessforge/internal/analyzer"
	"reflect"
	"strings"
	"testing"
)

func TestDetectedLanguagesUsedAndAdjustmentsGuidePlan(t *testing.T) {
	for _, test := range []struct {
		input string
		want  []string
	}{
		{"\n", []string{"Go"}},
		{"2\n1,3\n", []string{"Go", "Python"}},
	} {
		analysis := &analyzer.Analysis{Project: "sample", Languages: []analyzer.Finding{{Value: "Go"}}}
		session := setupSession{reader: bufio.NewReader(strings.NewReader(test.input)), output: &bytes.Buffer{}}
		got, err := session.adjustDetectedLanguages(analysis)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("languages = %v, %v", got, err)
		}
		plan, err := buildSetupPlan(t.TempDir(), analysis, nil, "", setupProvider{}, []string{"codex"}, setupAIProposal{})
		if err != nil || !reflect.DeepEqual(plan.Harness.Project.Languages, test.want) {
			t.Fatalf("plan languages = %v, %v", plan.Harness.Project.Languages, err)
		}
	}
}
