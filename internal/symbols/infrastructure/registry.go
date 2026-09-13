package infrastructure

import (
	"fmt"
	"harnessforge/internal/symbols/application"
	"strings"
)

type Registry struct{}

func (Registry) Resolve(language string) (application.Parser, error) {
	switch strings.ToLower(language) {
	case "go":
		return Go{}, nil
	case "java":
		return Java{}, nil
	default:
		return nil, fmt.Errorf("unsupported source language %q; supported languages are go and java", language)
	}
}
