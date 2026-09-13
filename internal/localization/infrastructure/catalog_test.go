package infrastructure

import (
	"harnessforge/internal/localization/domain"
	"testing"
)

func TestMissingTranslationFallsBackToEnglish(t *testing.T) {
	catalog := Catalog{}
	if got := catalog.Text(domain.Language("unknown"), "output.valid"); got != "Valid Harness IR: %s\n" {
		t.Fatal(got)
	}
	if got := catalog.Text(domain.BrazilianPortuguese, "untranslated diagnostic"); got != "untranslated diagnostic" {
		t.Fatal(got)
	}
}
