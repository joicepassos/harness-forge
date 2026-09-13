package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executeRoot(args ...string) (string, error) {
	root := newRootCommand()
	output := &bytes.Buffer{}
	root.SetOut(output)
	root.SetErr(output)
	root.SetArgs(args)
	err := root.Execute()
	return output.String(), err
}

func TestEnglishIsTheDefaultLanguage(t *testing.T) {
	output, err := executeRoot("--help")
	if err != nil || !strings.Contains(output, "Analyze a repository") {
		t.Fatalf("default help = %q, %v", output, err)
	}
}

func TestBrazilianPortugueseLocalizesCommandHelp(t *testing.T) {
	output, err := executeRoot("--language", "pt-BR", "--help")
	if err != nil || !strings.Contains(output, "Analisar um repositório") {
		t.Fatalf("Portuguese help = %q, %v", output, err)
	}
}

func TestBrazilianPortugueseLocalizesACommonError(t *testing.T) {
	_, err := executeRoot("--language", "pt-BR", "analyze", "--format", "invalid")
	if err == nil || !strings.Contains(err.Error(), "formato não suportado") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBrazilianPortugueseLocalizesACommonCommandOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harness.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nproject:\n  name: sample\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := executeRoot("--language", "pt-BR", "validate", path)
	if err != nil || !strings.Contains(output, "Harness IR válido") {
		t.Fatalf("Portuguese output = %q, %v", output, err)
	}
}

func TestUnsupportedLanguageReturnsClearError(t *testing.T) {
	_, err := executeRoot("--language", "fr", "version")
	if err == nil || !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInvalidLanguageCannotBypassValidationWithHelp(t *testing.T) {
	for _, args := range [][]string{{"--language", "fr", "--help"}, {"validate", "--help", "--language=fr"}, {"--language=", "version"}} {
		if _, err := executeRoot(args...); err == nil {
			t.Fatalf("accepted invalid language: %v", args)
		}
	}
}

func TestLocalizedHelpIncludesHeadingsAndFlags(t *testing.T) {
	output, err := executeRoot("analyze", "--language=pt-BR", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Uso:", "Opções:", "Incluir metadados Git", "Analisar um repositório"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in %s", want, output)
		}
	}
}
