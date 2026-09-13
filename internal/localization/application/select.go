package application

import (
	"fmt"
	"harnessforge/internal/localization/domain"
)

type Catalog interface {
	Supports(domain.Language) bool
}

func Select(catalog Catalog, value string) (domain.Language, error) {
	language := domain.Language(value)
	if !catalog.Supports(language) {
		return "", fmt.Errorf("unsupported language %q; supported languages are en, pt-BR and es", value)
	}
	return language, nil
}
