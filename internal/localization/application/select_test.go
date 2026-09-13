package application_test

import (
	"harnessforge/internal/localization/application"
	"harnessforge/internal/localization/domain"
	"harnessforge/internal/localization/infrastructure"
	"strings"
	"testing"
)

func TestSelectSupportsEnglishAndBrazilianPortuguese(t *testing.T) {
	catalog := infrastructure.Catalog{}
	for _, input := range []string{"en", "pt-BR", "es"} {
		selected, err := application.Select(catalog, input)
		if err != nil || selected != domain.Language(input) {
			t.Fatalf("Select(%q) = %q, %v", input, selected, err)
		}
	}
}

func TestSelectRejectsUnsupportedLanguage(t *testing.T) {
	_, err := application.Select(infrastructure.Catalog{}, "pt")
	if err == nil || !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("unexpected error: %v", err)
	}
}
