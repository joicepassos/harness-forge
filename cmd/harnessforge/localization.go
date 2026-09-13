package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/localization/application"
	"harnessforge/internal/localization/domain"
	localizationinfra "harnessforge/internal/localization/infrastructure"
	"strings"
)

type localizer struct {
	language domain.Language
	catalog  localizationinfra.Catalog
}

func newLocalizer(language string) (*localizer, error) {
	catalog := localizationinfra.Catalog{}
	selected, err := application.Select(catalog, language)
	if err != nil {
		return nil, err
	}
	return &localizer{language: selected, catalog: catalog}, nil
}

func (l *localizer) text(key string) string { return l.catalog.Text(l.language, key) }

func localizerFor(cmd *cobra.Command) (*localizer, error) {
	flag := cmd.Root().PersistentFlags().Lookup("language")
	if flag == nil {
		return newLocalizer("en")
	}
	language, err := cmd.Root().PersistentFlags().GetString("language")
	if err != nil {
		return nil, err
	}
	return newLocalizer(language)
}

func (l *localizer) printf(cmd *cobra.Command, key string, values ...any) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(), l.text(key), values...)
	return err
}

func applyLanguage(root *cobra.Command, l *localizer) {
	root.Short = l.text("root.short")
	keys := map[string]string{"version": "version.short", "init": "init.short", "analyze": "analyze.short", "ask": "ask.short", "config": "config.short", "validate": "validate.short", "review": "review.short", "context": "context.short", "skill": "skill.short", "eval": "eval.short", "doctor": "doctor.short", "github": "github.short", "drift": "drift.short", "config set": "config.set.short", "config get": "config.get.short", "context explain": "context.explain.short", "skill discover": "skill.discover.short", "skill generate": "skill.generate.short", "eval run": "eval.run.short", "eval compare": "eval.compare.short", "github learn": "github.learn.short"}
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		path := strings.TrimSpace(strings.TrimPrefix(command.CommandPath(), root.Name()))
		if key, ok := keys[path]; ok {
			command.Short = l.text(key)
		}
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(root)
}
