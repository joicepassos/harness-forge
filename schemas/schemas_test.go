package schemas

import "testing"

func TestAnalysisSchema(t *testing.T) {
	for _, bad := range []string{
		`{"architecture":[],"architecture":[],"patterns":[]}`,
		`{"architecture":["Java"],"patterns":[]}`,
		`{"architecture":[],"patterns":[]} {}`,
	} {
		if err := Validate("architecture-analysis.schema.json", []byte(bad)); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	if err := Validate("architecture-analysis.schema.json", []byte(`{"architecture":[],"patterns":[]}`)); err != nil {
		t.Fatal(err)
	}
}
