package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetupReviewRendersUTF8FileContentsAsText(t *testing.T) {
	content := []byte("version: 1\n# Revis\u00e3o do projeto\n")
	for _, portuguese := range []bool{false, true} {
		var output bytes.Buffer
		plan := setupPlan{Files: []setupOutputFile{{Path: ".harness/harness.yaml", Content: content}}}
		if portuguese {
			showSetupReview(setupLocalizedWriter{output: &output}, plan)
		} else {
			showSetupReview(&output, plan)
		}
		if !bytes.Contains(output.Bytes(), content) || strings.Contains(output.String(), "[118 101 114") {
			t.Fatalf("file preview is not readable text: %s", output.String())
		}
	}
}
