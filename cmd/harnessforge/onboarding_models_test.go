package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestSetupModelSelections(t *testing.T) {
	for _, provider := range []string{"openai", "deepseek", "gemini", "groq", "ollama"} {
		for _, input := range []string{"\n", "2\n", "bad\n0\ncustom-model\n"} {
			var output bytes.Buffer
			s := setupSession{reader: bufio.NewReader(strings.NewReader(input)), output: &output}
			model, err := s.chooseModel(provider)
			if err != nil || model == "" {
				t.Fatalf("%s: %q %v", provider, model, err)
			}
			if strings.HasPrefix(input, "bad") && model != "custom-model" {
				t.Fatalf("advanced model: %q", model)
			}
			if input == "2\n" && model != setupModels(provider)[1] {
				t.Fatalf("numbered model: %q", model)
			}
		}
	}
}
