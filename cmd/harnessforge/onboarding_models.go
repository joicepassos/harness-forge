package main

import (
	"fmt"
	"harnessforge/internal/llm"
	"strconv"
	"strings"
)

// These chat-completions choices are curated; account access and local installs vary.
func setupModels(provider string) []string {
	choices := map[string][]string{
		"openai":   {"gpt-4o-mini", "gpt-4.1-mini"},
		"deepseek": {"deepseek-flash", "deepseek-v4-pro"},
		"gemini":   {"gemini-2.5-flash", "gemini-2.5-pro"},
		"groq":     {"openai/gpt-oss-120b", "openai/gpt-oss-20b"},
		"ollama":   {"llama3.2", "qwen3"},
	}[provider]
	if recommended := llm.DefaultModel(provider); recommended != "" && len(choices) > 0 {
		choices[0] = recommended
	}
	return choices
}

func (s setupSession) chooseModel(provider string) (string, error) {
	models := setupModels(provider)
	if len(models) == 0 {
		return "", fmt.Errorf("unsupported provider %q", provider)
	}
	if s.interactive {
		labels := append(append([]string(nil), models...), "Advanced: enter a model identifier")
		values := append(append([]string(nil), models...), "advanced")
		model, err := s.formSelect("Model", labels, values, models[0])
		if err != nil {
			return "", err
		}
		if model == "advanced" {
			return s.formInput("Model identifier", false)
		}
		return model, nil
	}
	fmt.Fprintln(s.output, "Models (availability depends on your account or local installation):")
	for i, model := range models {
		fmt.Fprintf(s.output, "  %d) %s\n", i+1, model)
	}
	fmt.Fprintln(s.output, "  0) Advanced: enter a model identifier")
	for {
		answer, err := s.ask("Model [1]: ")
		if err != nil {
			return "", err
		}
		if answer == "" {
			return models[0], nil
		}
		index, err := strconv.Atoi(answer)
		if err == nil && index >= 1 && index <= len(models) {
			return models[index-1], nil
		}
		if answer == "0" {
			model, err := s.ask("Model identifier: ")
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(model) != "" {
				return model, nil
			}
		}
		fmt.Fprintf(s.output, "Choose a model number from 1 to %d, or 0 for advanced input.\n", len(models))
	}
}
