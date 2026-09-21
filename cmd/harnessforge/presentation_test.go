package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelpHasBrandAndKeepsLocalizedHeadings(t *testing.T) {
	for _, test := range []struct{ language, heading string }{
		{"en", "Usage:"},
		{"pt-BR", "Uso:"},
		{"es", "Uso:"},
	} {
		output, err := executeRoot("--language="+test.language, "--help")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output, "[HF] HarnessForge") || !strings.Contains(output, test.heading) || strings.Contains(output, "\x1b[") {
			t.Fatalf("unexpected %s help: %q", test.language, output)
		}
	}
}

func TestGuidedSetupHeaderHasBrandWithoutEscapeCodesInCapturedOutput(t *testing.T) {
	var output bytes.Buffer
	if err := runGuidedInit(context.Background(), strings.NewReader(""), &output, t.TempDir(), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "[HF] HarnessForge\nHarnessForge setup\n") || strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("unexpected install header: %q", output.String())
	}
}

func TestStatusStylesOnlyInteractiveOutput(t *testing.T) {
	plain := presentation{}.status("success", "Created file")
	if plain != "Created file" {
		t.Fatalf("plain status = %q", plain)
	}
	colored := presentation{colorful: true}.status("success", "Created file")
	if !strings.Contains(colored, "✓") || !strings.Contains(colored, "\x1b[") || !strings.Contains(colored, "Created file") {
		t.Fatalf("colored status = %q", colored)
	}
}

func TestASCIIStatusAndBrand(t *testing.T) {
	style := presentation{colorful: true, asciiOnly: true}
	for _, value := range []string{style.brand(), style.status("success", "Done"), style.status("error", "Failed")} {
		if strings.ContainsAny(value, "⚒✓✗") {
			t.Fatalf("non-ASCII symbol in %q", value)
		}
	}
	if !strings.Contains(style.brand(), "[HF] HarnessForge") {
		t.Fatal("brand missing ASCII fallback")
	}
}

func TestAnalysisStylesOnlyHeadings(t *testing.T) {
	input := "HarnessForge\n\nProject\nExample\n\nLanguages\n- Go\n\nSummary\nFiles: 1\n"
	if got := (presentation{}).analysis(input); got != input {
		t.Fatalf("plain analysis changed: %q", got)
	}
	styled := (presentation{colorful: true}).analysis(input)
	if !strings.Contains(styled, "⚒ HarnessForge") || strings.Count(styled, "\x1b[") < 3 || !strings.Contains(styled, "Files: 1") {
		t.Fatalf("analysis styling missing: %q", styled)
	}
}
