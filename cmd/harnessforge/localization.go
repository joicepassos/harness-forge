package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	return newLocalizer(flag.Value.String())
}

type languageValue string

func (v *languageValue) String() string { return string(*v) }
func (v *languageValue) Type() string   { return "string" }
func (v *languageValue) Set(value string) error {
	if _, err := newLocalizer(value); err != nil {
		return err
	}
	*v = languageValue(value)
	return nil
}

func (l *localizer) printf(cmd *cobra.Command, key string, values ...any) error {
	message := fmt.Sprintf(l.text(key), values...)
	if key == "output.created" || key == "output.valid" || key == "output.skill" {
		message = presentationFor(cmd.OutOrStdout()).status("success", message)
	} else if key == "output.exists" {
		message = presentationFor(cmd.OutOrStdout()).status("warning", message)
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), message)
	return err
}

func applyLanguage(root *cobra.Command, l *localizer) {
	root.Short = l.text("root.short")
	root.SetUsageTemplate(l.catalog.Presentation(l.language, usageTemplate))
	keys := map[string]string{"version": "version.short", "init": "init.short", "analyze": "analyze.short", "ask": "ask.short", "config": "config.short", "validate": "validate.short", "review": "review.short", "context": "context.short", "skill": "skill.short", "eval": "eval.short", "doctor": "doctor.short", "github": "github.short", "drift": "drift.short", "embedding": "embedding.short", "symbols": "symbols.short", "config set": "config.set.short", "config get": "config.get.short", "context explain": "context.explain.short", "skill discover": "skill.discover.short", "skill generate": "skill.generate.short", "eval run": "eval.run.short", "eval compare": "eval.compare.short", "github learn": "github.learn.short", "embedding create": "embedding.create.short", "embedding similarity": "embedding.similarity.short"}
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		path := strings.TrimSpace(strings.TrimPrefix(command.CommandPath(), root.Name()))
		if key, ok := keys[path]; ok {
			command.Short = l.text(key)
		}
		command.Short = l.catalog.Presentation(l.language, command.Short)
		command.InitDefaultHelpFlag()
		translateFlag := func(flag *pflag.Flag) {
			flag.Usage = l.catalog.Presentation(l.language, flag.Usage)
		}
		command.LocalFlags().VisitAll(translateFlag)
		command.PersistentFlags().VisitAll(translateFlag)
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(root)
}

const usageTemplate = `Usage:
  {{.UseLine}}
{{if .HasAvailableSubCommands}}
Available Commands:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}`
