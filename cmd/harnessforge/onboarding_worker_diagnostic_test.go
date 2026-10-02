package main

import (
	"errors"
	"strings"
	"testing"
)

func TestWorkerStartupDiagnosticsAreSpecificAndSafe(t *testing.T) {
	for _, message := range []string{"could not read worker input", "invalid worker input", "worker context does not match the authorized run", "worker document does not match the authorized run"} {
		got := setupWorkerStartupError(errors.New(message))
		if !strings.Contains(got, "No provider request was sent.") || strings.Contains(got, "could not complete safely") {
			t.Fatalf("ambiguous startup diagnosis: %s", got)
		}
	}
	if strings.Contains(setupWorkerStartupError(errors.New("private-key-and-document")), "private-key") {
		t.Fatal("raw internal error leaked")
	}
}
