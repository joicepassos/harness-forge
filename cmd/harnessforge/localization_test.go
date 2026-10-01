package main

import (
	"bytes"
	"io"
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

func TestHelpAndVersionWriteToStdoutByDefault(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"version"}} {
		stdoutReader, stdoutWriter, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		stderrReader, stderrWriter, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		previousOut, previousErr := os.Stdout, os.Stderr
		os.Stdout, os.Stderr = stdoutWriter, stderrWriter
		command := newRootCommand()
		command.SetArgs(args)
		executeErr := command.Execute()
		os.Stdout, os.Stderr = previousOut, previousErr
		stdoutWriter.Close()
		stderrWriter.Close()
		output, readErr := io.ReadAll(stdoutReader)
		if readErr != nil {
			t.Fatal(readErr)
		}
		errors, readErr := io.ReadAll(stderrReader)
		if readErr != nil {
			t.Fatal(readErr)
		}
		stdoutReader.Close()
		stderrReader.Close()
		if executeErr != nil || len(output) == 0 || len(errors) != 0 {
			t.Fatalf("%v: stdout=%q stderr=%q error=%v", args, output, errors, executeErr)
		}
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

func TestSpanishHelpErrorsAndOutput(t *testing.T) {
	output, err := executeRoot("analyze", "--language=es", "--help")
	if err != nil || !strings.Contains(output, "Analizar un repositorio") || !strings.Contains(output, "Opciones:") {
		t.Fatalf("%s: %v", output, err)
	}
	_, err = executeRoot("--language=es", "analyze", "--format=invalid")
	if err == nil || !strings.Contains(err.Error(), "formato no compatible") {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "harness.yaml")
	data := []byte("version: 1\nproject:\n  name: sample\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	output, err = executeRoot("validate", "--language=es", path)
	if err != nil || !strings.Contains(output, "Harness IR válido") {
		t.Fatalf("%s: %v", output, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("validation changed the source")
	}
}

func TestLocalePreservesMachineReadableOutput(t *testing.T) {
	repository := t.TempDir()
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("# Sample\n"), 0600); err != nil {
		t.Fatal(err)
	}
	baseline, err := executeRoot("analyze", repository, "--format=json")
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"pt-BR", "es"} {
		output, err := executeRoot("analyze", repository, "--format=json", "--language="+locale)
		if err != nil || output != baseline {
			t.Fatalf("locale %s changed JSON: %v", locale, err)
		}
	}
}
